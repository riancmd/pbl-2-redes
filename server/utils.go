package main

import (
	"PlanoZ/models"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
)

// --- Funções Utilitárias (Rede, Jogo, etc.) ---

// lidarPing é o servidor UDP. Ele ouve por "ping" e responde "pong".
func (s *Server) lidarPing(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			color.Red("Erro ao ler do UDP: %v", err)
			continue
		}

		// (Não precisa de JSON, apenas checa a string)
		if string(buffer[:n]) == "ping" {
			// Envia a resposta "pong" de volta para o endereço remetente
			_, err = conn.WriteToUDP([]byte("pong"), addr)
			if err != nil {
				color.Red("Erro ao enviar pong UDP para %s: %v", addr.String(), err)
			}
		}
	}
}

// sortearCartas (Original)
func (s *Server) sortearCartas(playerID string) []models.Tanque {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	n := len(pacote_1)
	indices := r.Perm(n)[:5]
	cartasSorteadas := make([]models.Tanque, 0, 5)

	for _, i := range indices {
		cartasSorteadas = append(cartasSorteadas, pacote_1[i])
	}
	for i := range cartasSorteadas {
		cartasSorteadas[i].Id_jogador = playerID
	}
	return cartasSorteadas
}

// --- Helpers de Comunicação ---

// (Helper: Enviar para Cliente via Redis)
func (s *Server) sendToClient(replyChannel, tipo string, data interface{}) {
	resp := models.RespostaGenericaCliente{
		Tipo: tipo,
		Data: data,
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		color.Red("Erro ao serializar resposta para %s: %v", replyChannel, err)
		return
	}
	if err := s.redisClient.RPush(s.ctx, replyChannel, respBytes).Err(); err != nil {
		color.Red("Erro ao enviar (RPUSH) para %s: %v", replyChannel, err)
	}
}

// (Helper: Enviar para outro Servidor via API)
func (s *Server) sendToHost(host, endpoint string, payload interface{}) error {
	// Não envia para si mesmo (evita deadlock)
	if host == s.HostAPI {
		color.Red("ERRO LÓGICO: sendToHost chamado para si mesmo (%s)", host)
		return fmt.Errorf("tentativa de sendToHost para si mesmo")
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", host, endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("servidor %s respondeu com status %d", host, resp.StatusCode)
	}
	return nil
}

// (Helper: Enviar para o Líder)
func (s *Server) sendToLeader(endpoint string, payload interface{}) error {
	s.muLeader.RLock()
	leaderID := s.currentLeader
	s.muLeader.RUnlock()

	if leaderID == "" {
		return fmt.Errorf("líder ainda não eleito")
	}

	leaderHost, ok := s.serverList[leaderID]
	if !ok {
		return fmt.Errorf("líder %s desconhecido ou offline", leaderID)
	}

	if leaderID == s.ID {
		color.Red("ERRO LÓGICO: sendToLeader chamado pelo próprio líder. Use a função local.")
		return fmt.Errorf("lógica inválida: sendToLeader chamado pelo próprio líder")
	}

	return s.sendToHost(leaderHost, endpoint, payload)
}

// (Helper: Enviar para todos os outros servidores VIVOS)
func (s *Server) broadcastToServers(endpoint string, payload interface{}) {
	s.muLiveServers.RLock()
	// Copia o mapa para evitar lock durante as chamadas de rede
	live := make(map[string]bool)
	for id, isLive := range s.liveServers {
		if isLive {
			live[id] = true
		}
	}
	s.muLiveServers.RUnlock()

	for id := range live {
		if id == s.ID { // Não envia para si mesmo
			continue
		}
		host := s.serverList[id]

		go func(h, e string, p interface{}) {
			if err := s.sendToHost(h, e, p); err != nil {
				color.Red("Falha ao transmitir para %s: %v", h, err)
			}
		}(host, endpoint, payload)
	}
}

// (Helper: Ler Env Var)
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
