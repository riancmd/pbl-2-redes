package cluster

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/infrastructure/bully"
	"pbl-2-redes/internal/models"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Representa o Client (outbound) daquele servidor específico dentro do Cluster
type Client struct {
	peers         []int
	bullyElection *bully.bullyElection // erro no package por algum motivo [CONSERTAR]
	httpClient    *http.Client
}

// Cria um novo Client no Cluster
func New(allPeers []int, port int) *Client {
	// Guarda lista de peers no cluster
	var myPeers []int

	// Remove a porta da lista
	for _, address := range allPeers {
		if address != port {
			myPeers = append(myPeers, address)
		}
	}

	client := Client{
		peers:         myPeers,
		bullyElection: bully.New(port, myPeers),
		httpClient:    &http.Client{Timeout: 5 * time.Second},
	}

	// Faz eleição
	client.bullyElection.startElection()

	return &client
}

// Faz a sincronização do banco de dados
// Usado no início, pelo líder
func (c *Client) SyncCards() ([]models.Booster, error) {
	// dá um GET nas cartas
	resp, err := c.httpClient.Get("http://localhost:" + strconv.Itoa(c.peers[0]) + "/cards") // Endereço temporário, resolver

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var cards []models.Booster

	json.NewDecoder(resp.Body).Decode(&cards)
	return cards, nil
}

// Sincroniza o enqueue na fila de batalha
func (c *Client) BattleEnqueue(UID uuid.UUID) error {
	return nil
}

// Sincroniza o dequeue na fila de batalha
func (c *Client) BattleDequeue() error {
	return nil
}

// Sincroniza o enqueue na fila de troca
func (c *Client) TradingEnqueue(UID uuid.UUID) error {
	return nil
}

// Sincroniza o dequeue na fila de troca
func (c *Client) TradingDequeue() error {
	return nil
}

// Sincroniza nova batalha
func (c *Client) MatchNew(match models.Match) error {
	return nil
}

// Sincroniza fim de batalha
func (c *Client) MatchEnd(ID uuid.UUID) error {
	return nil
}

// Sincroniza compra de carta
func (c *Client) BuyBooster(boosterID int) error {
	return nil
}

// Sincroniza troca de carta
func (c *Client) TradeCard() error {
	return nil
}

// Sincroniza criação de usuários, evitando cópias
func (c *Client) UserNew(username string) error {
	return nil
}
