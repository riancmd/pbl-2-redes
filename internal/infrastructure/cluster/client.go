package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"pbl-2-redes/internal/infrastructure/bully"
	"pbl-2-redes/internal/models"
	"strconv"
	"time"
)

// Representa o Client (outbound) daquele servidor específico dentro do Cluster
type Client struct {
	peers         []int
	bullyElection *bully.BullyElection // erro no package por algum motivo [CONSERTAR]
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
	client.bullyElection.StartElection()

	return &client
}

// Verifica se é líder (uso externo)
func (c *Client) IsLeader() bool {
	return c.bullyElection.IsLeader()
}

// Faz a sincronização do banco de dados
// Usado no início, pelos seguidores
func (c *Client) SyncCards() ([]models.Booster, error) {
	// dá um GET nas cartas
	resp, err := c.httpClient.Get("http://localhost:" + strconv.Itoa(c.bullyElection.GetLeader()) + "/internal/cards") // Endereço temporário, resolver

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var cards []models.Booster

	json.NewDecoder(resp.Body).Decode(&cards)
	return cards, nil
}

// Sincroniza o enqueue na fila de batalha
func (c *Client) BattleEnqueue(UID string) error {
	// Encapsula dados em JSON
	jsonData, err := json.Marshal(UID)

	if err != nil {
		log.Fatalf("error while converting to json: %v", err)
	}
	// Dá um POST na queue
	resp, err := c.httpClient.Post(
		"http://localhost:"+strconv.Itoa(c.bullyElection.GetLeader())+"/internal/battle_queue",
		"application/json",
		bytes.NewBuffer(jsonData)) // Endereço temporário, resolver

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Verifica o código enviado de resposta
	if resp.StatusCode != http.StatusAccepted {

		// Lê o erro
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			// Se não conseguir ler o corpo, retorne pelo menos o status
			return fmt.Errorf("couldn't read message: status %s", resp.Status)
		}

		// Retorna o erro
		return fmt.Errorf("status: %s. msg: %s", resp.Status, string(bodyBytes))
	}

	return nil
}

// Sincroniza o dequeue na fila de batalha
func (c *Client) BattleDequeue() error {
	return nil
}

// Sincroniza o enqueue na fila de troca
func (c *Client) TradingEnqueue(UID string) error {
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
func (c *Client) MatchEnd(ID string) error {
	return nil
}

// Sincroniza compra de carta
func (c *Client) BuyBooster(boosterID int) error {
	return nil
}

// Sincroniza troca de carta
func (c *Client) TradeCard(action string, info string, card models.Card) error {
	leader := c.bullyElection.IsLeader()
	if leader {
		c.BroadcastToPeers(action, info)
		return nil
	}
	err := c.AskLeader(action, info)
	return err
}

// Sincroniza criação de usuários, evitando cópias
func (c *Client) UserNew(username string) error {
	return nil
}

// Solicita ao líder permissão
func (c *Client) AskLeader(action string, info string) error {
	return nil
}

// Ordena aos peers que façam algo
func (c *Client) BroadcastToPeers(action string, info string) error {
	return nil
}

// Atualiza partida
func (c *Client) UpdateMatch(match models.Match) error {
	return nil
}
