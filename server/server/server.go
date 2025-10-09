package server

import (
	"bytes"
	"card-game/models"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Server representa uma instância de um servidor de jogo no cluster.
type Server struct {
	State      *models.ServerState
	Peers      []string
	MyAddress  string
	HttpClient *http.Client
	CardPool   []models.Card
}

// NewServer cria e inicializa uma nova instância do servidor.
func NewServer(peers []string, myAddress string, startPacks int, cardPool []models.Card) *Server {
	return &Server{
		State: &models.ServerState{
			QuantidadePacotes: startPacks,
		},
		Peers:      peers,
		MyAddress:  myAddress,
		HttpClient: &http.Client{Timeout: 3 * time.Second},
		CardPool:   cardPool,
	}
}

// StartPurchaseTransaction orquestra o processo de 2-Phase Commit para uma compra.
// Esta função é chamada no servidor que atua como Coordenador.
func (s *Server) StartPurchaseTransaction(clientID string) (bool, models.Card) {
	transactionID := uuid.New().String()
	log.Printf("[COORDENADOR %s] Iniciando transação de compra %s", s.MyAddress, transactionID)
	payload := models.PurchasePayload{TransactionID: transactionID, ClientID: clientID}

	// Fase 1: Envia solicitações de "prepare" para todos os outros servidores (peers).
	var wg sync.WaitGroup
	results := make(chan bool, len(s.Peers))
	for _, peer := range s.Peers {
		if peer != s.MyAddress {
			wg.Add(1)
			go s.broadcast(peer, "/prepare-purchase", payload, &wg, results)
		}
	}
	wg.Wait()
	close(results)

	// Fase 2: Tomada de Decisão.
	// Verifica se o próprio coordenador e todos os peers estão prontos.
	canSelfPrepare := s.CanPreparePurchase()
	allPrepared := canSelfPrepare
	for result := range results {
		if !result {
			allPrepared = false
			break
		}
	}

	if !allPrepared {
		log.Printf("[COORDENADOR %s] Transação %s ABORTADA.", s.MyAddress, transactionID)
		go s.broadcastToAll("/abort-purchase", payload)
		return false, models.Card{}
	}

	log.Printf("[COORDENADOR %s] Transação %s será COMMITADA.", s.MyAddress, transactionID)
	s.CommitPurchase()
	go s.broadcastToAll("/commit-purchase", payload)

	// Sorteia a carta para o cliente após o sucesso do commit.
	randomIndex := rand.Intn(len(s.CardPool))
	wonCard := s.CardPool[randomIndex]
	log.Printf("[COORDENADOR %s] Cliente %s tirou a carta: %s", s.MyAddress, clientID, wonCard.Nome)

	return true, wonCard
}

// broadcastToAll envia uma mensagem para todos os peers, exceto para si mesmo.
func (s *Server) broadcastToAll(endpoint string, payload models.PurchasePayload) {
	for _, peer := range s.Peers {
		if peer != s.MyAddress {
			go s.broadcast(peer, endpoint, payload, nil, nil)
		}
	}
}

// broadcast envia uma única requisição HTTP para um peer.
func (s *Server) broadcast(peer, endpoint string, payload models.PurchasePayload, wg *sync.WaitGroup, results chan<- bool) {
	if wg != nil {
		defer wg.Done()
	}
	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", peer+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		if results != nil {
			results <- false
		}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HttpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Falha ao comunicar com peer %s em %s. Erro: %v", peer, endpoint, err)
		if results != nil {
			results <- false
		}
		return
	}
	defer resp.Body.Close()
	if results != nil {
		results <- true
	}
}

// CommitPurchase efetua a decrementação do estoque de pacotes.
func (s *Server) CommitPurchase() {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	if s.State.QuantidadePacotes > 0 {
		s.State.QuantidadePacotes--
	}
	log.Printf("[%s] COMMIT. Estoque agora: %d", s.MyAddress, s.State.QuantidadePacotes)
}

// CanPreparePurchase verifica se o servidor tem estoque para a venda.
func (s *Server) CanPreparePurchase() bool {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	return s.State.QuantidadePacotes > 0
}
