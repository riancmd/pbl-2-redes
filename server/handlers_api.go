package main

import (
	"PlanoZ/models"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

// --- Handlers da API REST (Gin) ---

func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthCheckResponse{
		Status:   "OK",
		ServerID: s.ID,
		IsLeader: s.isLeader(),
	})
}

// --- Handlers de Sincronização ---

// (Líder) Recebe notificação de conexão de um seguidor
func (s *Server) handleLeaderConnect(c *gin.Context) {
	if !s.isLeader() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Eu não sou o líder"})
		return
	}

	var req models.LeaderConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// 1. Atualizar a lista de jogadores
	s.muPlayers.Lock()
	oldInfo, exists := s.playerList[req.PlayerID]

	playerInfo := PlayerInfo{
		ServerID:     req.ServerID,
		ServerHost:   s.serverList[req.ServerID],
		ReplyChannel: req.CanalResposta,
	}
	s.playerList[req.PlayerID] = playerInfo
	s.muPlayers.Unlock()

	color.Cyan("LÍDER: Jogador %s registrado no servidor %s", req.PlayerID, req.ServerID)

	// 2. Transmitir a atualização para TODOS os servidores
	// Se existia, notifica a remoção do server antigo
	if exists && oldInfo.ServerID != req.ServerID {
		updateRemove := models.UpdatePlayerListRequest{
			PlayerID: req.PlayerID, ServerID: oldInfo.ServerID, Acao: "remove",
		}
		s.broadcastToServers("/players/update", updateRemove)
	}
	// Notifica a adição no novo server
	updateReq := models.UpdatePlayerListRequest{
		PlayerID: req.PlayerID, ServerID: req.ServerID, CanalResposta: req.CanalResposta, Acao: "add",
	}
	s.broadcastToServers("/players/update", updateReq)

	c.JSON(http.StatusOK, gin.H{"message": "Jogador registrado pelo líder"})
}

// (Seguidor) Recebe atualização da lista de jogadores do líder
func (s *Server) handlePlayerUpdate(c *gin.Context) {
	var req models.UpdatePlayerListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muPlayers.Lock()
	if req.Acao == "add" {
		s.playerList[req.PlayerID] = PlayerInfo{
			ServerID:     req.ServerID,
			ServerHost:   s.serverList[req.ServerID],
			ReplyChannel: req.CanalResposta,
		}
		color.Cyan("SEGUIDOR: Lista de jogadores atualizada, ADD %s", req.PlayerID)
	} else if req.Acao == "remove" {
		// Apenas remove se o jogador estiver no servidor que estamos removendo
		if info, ok := s.playerList[req.PlayerID]; ok && info.ServerID == req.ServerID {
			delete(s.playerList, req.PlayerID)
			color.Cyan("SEGUIDOR: Lista de jogadores atualizada, REMOVE %s", req.PlayerID)
		}
	}
	s.muPlayers.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "Lista de jogadores atualizada"})
}

// (Líder) Recebe pedido de compra de carta
func (s *Server) handleLeaderBuyCard(c *gin.Context) {
	if !s.isLeader() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Eu não sou o líder"})
		return
	}

	var req models.LeaderBuyCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muPlayers.RLock()
	playerInfo, ok := s.playerList[req.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador não encontrado"})
		return
	}

	s.muInventory.Lock()
	if s.pacoteCounter <= 0 {
		s.muInventory.Unlock()
		s.sendToClient(playerInfo.ReplyChannel, "Erro", models.RespostaErro{Erro: "Não há mais pacotes disponíveis"})
		c.JSON(http.StatusOK, gin.H{"message": "Estoque esgotado"})
		return
	}
	s.pacoteCounter--
	pacotesRestantes := s.pacoteCounter
	s.muInventory.Unlock()

	color.Cyan("LÍDER: Pacote vendido para %s. Restantes: %d", req.PlayerID, pacotesRestantes)

	// Transmitir atualização do inventário para todos
	invUpdate := models.UpdateInventoryRequest{PacotesRestantes: pacotesRestantes}
	s.broadcastToServers("/inventory/update", invUpdate)

	// Sortear cartas e enviar diretamente ao cliente
	cartas := s.sortearCartas(req.PlayerID)
	respSorteio := models.RespostaSorteio{
		Mensagem: "Sorteio realizado com sucesso!",
		Cartas:   cartas,
	}
	s.sendToClient(playerInfo.ReplyChannel, "Sorteio", respSorteio)

	c.JSON(http.StatusOK, gin.H{"message": "Compra processada"})
}

// (Seguidor) Recebe atualização do inventário
func (s *Server) handleInventoryUpdate(c *gin.Context) {
	var req models.UpdateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muInventory.Lock()
	s.pacoteCounter = req.PacotesRestantes
	s.muInventory.Unlock()

	color.Yellow("SEGUIDOR: Inventário atualizado. Pacotes restantes: %d", req.PacotesRestantes)
	c.JSON(http.StatusOK, gin.H{"message": "Inventário atualizado"})
}

// --- Handlers de Batalha (P2P entre Servidores) ---

// (Servidor J2) Recebe pedido (de S1) para iniciar batalha
func (s *Server) handleBattleInitiate(c *gin.Context) {
	var req models.BattleInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muPlayers.RLock()
	player2Info, ok := s.playerList[req.IdJogadorLocal]
	s.muPlayers.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador local (J2) não encontrado"})
		return
	}

	// Armazena a associação da batalha com o jogador local (J2) E o host (J1)
	s.muBatalhasPeer.Lock()
	s.batalhasPeer[req.IdBatalha] = peerBattleInfo{
		PlayerID: req.IdJogadorLocal,
		HostAPI:  req.HostServidor,
	}
	s.muBatalhasPeer.Unlock()

	// Notifica o cliente J2 que a batalha começou
	resp := models.RespostaInicioBatalha{
		Mensagem:  req.IdOponente, // Mensagem é o ID do Oponente
		IdBatalha: req.IdBatalha,
	}
	s.sendToClient(player2Info.ReplyChannel, "Inicio_Batalha", resp)

	color.Green("BATALHA (Peer J2): Batalha %s registrada para jogador %s. Host: %s", req.IdBatalha, req.IdJogadorLocal, req.HostServidor)
	c.JSON(http.StatusOK, gin.H{"message": "Batalha iniciada e registrada"})
}

// (Servidor J1 - Host) Recebe a carta (de S2) do Jogador 2
func (s *Server) handleBattleSubmitMove(c *gin.Context) {
	var req models.BattleSubmitMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// Encontra a batalha em andamento que este servidor está hospedando
	s.muBatalhas.RLock()
	batalha, ok := s.batalhas[req.IdBatalha]
	s.muBatalhas.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batalha não encontrada (no Host J1)"})
		return
	}

	// Envia a carta para a goroutine da batalha (iniciarBatalha)
	select {
	case batalha.CanalJ2 <- req.Carta:
		color.Green("BATALHA (Host J1): Recebida carta de J2 para batalha %s", req.IdBatalha)
		c.JSON(http.StatusOK, gin.H{"message": "Jogada recebida"})
	case <-time.After(5 * time.Second): // Timeout
		color.Red("BATALHA (Host J1): Timeout ao enviar carta de J2 para canal da batalha %s", req.IdBatalha)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Timeout interno"})
	}
}

// (Servidor J2) Recebe pedido (de S1) de jogada
func (s *Server) handleBattleRequestMove(c *gin.Context) {
	var req models.BattleRequestMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// 1. Descobrir quem é o J2 usando o BattleID
	s.muBatalhasPeer.RLock()
	peerInfo, ok := s.batalhasPeer[req.IdBatalha]
	s.muBatalhasPeer.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de batalha não encontrada (J2)"})
		return
	}

	// 2. Encontrar a informação de conexão do J2
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador (J2) não encontrado na lista"})
		return
	}

	// 3. Pede a carta ao cliente J2 (via Redis)
	resp := models.RespostaPedirCarta{Indice: req.Indice}
	s.sendToClient(player2Info.ReplyChannel, "Pedir_Carta", resp)

	color.Green("BATALHA (Peer J2): Pedido de carta (índice %d) enviado ao cliente %s", req.Indice, peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Pedido de jogada enviado"})
}

// (Servidor J2) Recebe (de S1) o resultado do turno
func (s *Server) handleBattleTurnResult(c *gin.Context) {
	var req models.BattleTurnResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muBatalhasPeer.RLock()
	peerInfo, ok := s.batalhasPeer[req.IdBatalha]
	s.muBatalhasPeer.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de batalha (J2) não encontrada"})
		return
	}

	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador (J2) não encontrado na lista"})
		return
	}

	// Envia o resultado do turno para o cliente J2 (via Redis)
	s.sendToClient(player2Info.ReplyChannel, "Turno_Realizado", req.Resultado)

	color.Green("BATALHA (Peer J2): Resultado do turno enviado ao cliente %s", peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Resultado do turno enviado"})
}

// (Servidor J2) Recebe (de S1) o fim da batalha
func (s *Server) handleBattleEnd(c *gin.Context) {
	var req models.BattleEndRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// 1. Descobrir quem é o J2 e LIMPAR o mapa
	s.muBatalhasPeer.Lock() // Usar Lock para poder deletar
	peerInfo, ok := s.batalhasPeer[req.IdBatalha]
	if ok {
		delete(s.batalhasPeer, req.IdBatalha)
	}
	s.muBatalhasPeer.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de batalha (J2) não encontrada (ou já encerrada)"})
		return
	}

	// 2. Encontrar a informação de conexão do J2
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		color.Yellow("BATALHA (Peer J2): Fim da batalha %s, mas J2 (%s) não encontrado. Associação limpa.", req.IdBatalha, peerInfo.PlayerID)
		c.JSON(http.StatusOK, gin.H{"message": "Fim da batalha processado, mas J2 não encontrado"})
		return
	}

	// 3. Envia o fim da batalha para o cliente J2 (via Redis)
	s.sendToClient(player2Info.ReplyChannel, "Fim_Batalha", req.Resultado)

	color.Green("BATALHA (Peer J2): Fim da batalha enviado ao cliente %s e associação limpa", peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Fim da batalha enviado"})
}
