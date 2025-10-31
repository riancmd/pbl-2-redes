package main

import (
	"PlanoZ/models"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

// handlers da api rest (gin)

// o outro server ta me perguntando se eu to vivo (health check)
func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthCheckResponse{
		Status:   "OK",
		ServerID: s.ID,
		IsLeader: s.isLeader(), // tbm aviso se eu sou o lider ou n
	})
}

// handlers de sincronização

// (so o lider executa) um seguidor (outro server) ta me avisando q um player conectou nele
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

	// atualiza a lista global de players
	s.muPlayers.Lock()
	oldInfo, exists := s.playerList[req.PlayerID] // ve se ele ja tava em outro server

	playerInfo := PlayerInfo{
		ServerID:     req.ServerID,
		ServerHost:   s.serverList[req.ServerID],
		ReplyChannel: req.CanalResposta,
	}
	s.playerList[req.PlayerID] = playerInfo
	s.muPlayers.Unlock()

	color.Cyan("LÍDER: Jogador %s registrado no servidor %s", req.PlayerID, req.ServerID)

	// avisa todo mundo sobre a mudança
	// se ele ja existia, avisa pra remover do server antigo
	if exists && oldInfo.ServerID != req.ServerID {
		updateRemove := models.UpdatePlayerListRequest{
			PlayerID: req.PlayerID, ServerID: oldInfo.ServerID, Acao: "remove",
		}
		s.broadcastToServers("/players/update", updateRemove)
	}
	// e avisa pra adicionar no server novo
	updateReq := models.UpdatePlayerListRequest{
		PlayerID: req.PlayerID, ServerID: req.ServerID, CanalResposta: req.CanalResposta, Acao: "add",
	}
	s.broadcastToServers("/players/update", updateReq)

	c.JSON(http.StatusOK, gin.H{"message": "Jogador registrado pelo líder"})
}

// (so o seguidor executa) o lider mandou uma atualizacao da lista de players
func (s *Server) handlePlayerUpdate(c *gin.Context) {
	var req models.UpdatePlayerListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muPlayers.Lock()
	if req.Acao == "add" {
		// adiciona o player na nossa copia local
		s.playerList[req.PlayerID] = PlayerInfo{
			ServerID:     req.ServerID,
			ServerHost:   s.serverList[req.ServerID],
			ReplyChannel: req.CanalResposta,
		}
		color.Cyan("SEGUIDOR: Lista de jogadores atualizada, ADD %s", req.PlayerID)
	} else if req.Acao == "remove" {
		// remove o player da nossa copia local
		// (so remove se for do server certo, pra evitar confusao)
		if info, ok := s.playerList[req.PlayerID]; ok && info.ServerID == req.ServerID {
			delete(s.playerList, req.PlayerID)
			color.Cyan("SEGUIDOR: Lista de jogadores atualizada, REMOVE %s", req.PlayerID)
		}
	}
	s.muPlayers.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "Lista de jogadores atualizada"})
}

// (so o lider executa) um seguidor ta pedindo pra eu processar uma compra de carta
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

	// acha o player pra saber pra qm responder
	s.muPlayers.RLock()
	playerInfo, ok := s.playerList[req.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador não encontrado"})
		return
	}

	// aqui eh a logica de negocio (unica fonte da verdade)
	s.muInventory.Lock()
	if s.pacoteCounter <= 0 {
		// sem estoque
		s.muInventory.Unlock()
		s.sendToClient(playerInfo.ReplyChannel, "Erro", models.RespostaErro{Erro: "Não há mais pacotes disponíveis"})
		c.JSON(http.StatusOK, gin.H{"message": "Estoque esgotado"})
		return
	}
	s.pacoteCounter-- // tira 1 do estoque
	pacotesRestantes := s.pacoteCounter
	s.muInventory.Unlock()

	color.Cyan("LÍDER: Pacote vendido para %s. Restantes: %d", req.PlayerID, pacotesRestantes)

	// avisa todo mundo (os outros seguidores) q o estoque mudou
	invUpdate := models.UpdateInventoryRequest{PacotesRestantes: pacotesRestantes}
	s.broadcastToServers("/inventory/update", invUpdate)

	// sorteia as cartas e manda direto pro cliente (via redis)
	cartas := s.sortearCartas(req.PlayerID)
	respSorteio := models.RespostaSorteio{
		Mensagem: "Sorteio realizado com sucesso!",
		Cartas:   cartas,
	}
	s.sendToClient(playerInfo.ReplyChannel, "Sorteio", respSorteio)

	c.JSON(http.StatusOK, gin.H{"message": "Compra processada"})
}

// (so o seguidor executa) o lider mandou atualizar o estoque de pacotes
func (s *Server) handleInventoryUpdate(c *gin.Context) {
	var req models.UpdateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	s.muInventory.Lock()
	s.pacoteCounter = req.PacotesRestantes // so atualiza o valor local
	s.muInventory.Unlock()

	color.Yellow("SEGUIDOR: Inventário atualizado. Pacotes restantes: %d", req.PacotesRestantes)
	c.JSON(http.StatusOK, gin.H{"message": "Inventário atualizado"})
}

// handlers de batalha (p2p entre servers)

// (server 2) o server 1 (host) ta me avisando q uma batalha comecou
func (s *Server) handleBattleInitiate(c *gin.Context) {
	var req models.BattleInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha o meu jogador local (j2)
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[req.IdJogadorLocal]
	s.muPlayers.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador local (J2) não encontrado"})
		return
	}

	// importante:
	// guarda no mapa 'batalhasPeer' o id da batalha,
	// o id do nosso player (j2) e a api do server 1 (host)
	// pra gnt saber pra qm responder depois
	s.muBatalhasPeer.Lock()
	s.batalhasPeer[req.IdBatalha] = peerBattleInfo{
		PlayerID: req.IdJogadorLocal,
		HostAPI:  req.HostServidor,
	}
	s.muBatalhasPeer.Unlock()

	// avisa o meu cliente (j2) q a batalha comecou
	resp := models.RespostaInicioBatalha{
		Mensagem:  req.IdOponente, // manda o id do oponente
		IdBatalha: req.IdBatalha,
	}
	s.sendToClient(player2Info.ReplyChannel, "Inicio_Batalha", resp)

	color.Green("BATALHA (Peer J2): Batalha %s registrada para jogador %s. Host: %s", req.IdBatalha, req.IdJogadorLocal, req.HostServidor)
	c.JSON(http.StatusOK, gin.H{"message": "Batalha iniciada e registrada"})
}

// (server 1 - host) o server 2 ta me devolvendo a carta q o j2 jogou
func (s *Server) handleBattleSubmitMove(c *gin.Context) {
	var req models.BattleSubmitMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha a batalha q eu to hospedando
	s.muBatalhas.RLock()
	batalha, ok := s.batalhas[req.IdBatalha]
	s.muBatalhas.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batalha não encontrada (no Host J1)"})
		return
	}

	// joga a carta do j2 no canal q a goroutine 'iniciarBatalha' ta esperando
	select {
	case batalha.CanalJ2 <- req.Carta:
		color.Green("BATALHA (Host J1): Recebida carta de J2 para batalha %s", req.IdBatalha)
		c.JSON(http.StatusOK, gin.H{"message": "Jogada recebida"})
	case <-time.After(5 * time.Second): // timeout
		color.Red("BATALHA (Host J1): Timeout ao enviar carta de J2 para canal da batalha %s", req.IdBatalha)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Timeout interno"})
	}
}

// (server 2) o server 1 (host) ta pedindo a jogada do meu player (j2)
func (s *Server) handleBattleRequestMove(c *gin.Context) {
	var req models.BattleRequestMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// descobre quem eh o meu player (j2) dessa batalha
	s.muBatalhasPeer.RLock()
	peerInfo, ok := s.batalhasPeer[req.IdBatalha]
	s.muBatalhasPeer.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de batalha não encontrada (J2)"})
		return
	}

	// acha o canal de resposta dele
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador (J2) não encontrado na lista"})
		return
	}

	// manda a msg pro meu cliente (j2) "ei, joga ai" (via redis)
	resp := models.RespostaPedirCarta{Indice: req.Indice}
	s.sendToClient(player2Info.ReplyChannel, "Pedir_Carta", resp)

	color.Green("BATALHA (Peer J2): Pedido de carta (índice %d) enviado ao cliente %s", req.Indice, peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Pedido de jogada enviado"})
}

// (server 2) o server 1 (host) ta mandando o resultado do turno
func (s *Server) handleBattleTurnResult(c *gin.Context) {
	var req models.BattleTurnResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha meu player (j2)
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

	// repassa o resultado pro meu cliente (j2) (via redis)
	s.sendToClient(player2Info.ReplyChannel, "Turno_Realizado", req.Resultado)

	color.Green("BATALHA (Peer J2): Resultado do turno enviado ao cliente %s", peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Resultado do turno enviado"})
}

// (server 2) o server 1 (host) ta avisando q a batalha acabou
func (s *Server) handleBattleEnd(c *gin.Context) {
	var req models.BattleEndRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha o j2 e limpa o mapa
	s.muBatalhasPeer.Lock()
	peerInfo, ok := s.batalhasPeer[req.IdBatalha]
	if ok {
		delete(s.batalhasPeer, req.IdBatalha) // limpeza!
	}
	s.muBatalhasPeer.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de batalha (J2) não encontrada (ou já encerrada)"})
		return
	}

	// acha o canal de resposta do j2
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		color.Yellow("BATALHA (Peer J2): Fim da batalha %s, mas J2 (%s) não encontrado. Associação limpa.", req.IdBatalha, peerInfo.PlayerID)
		c.JSON(http.StatusOK, gin.H{"message": "Fim da batalha processado, mas J2 não encontrado"})
		return
	}

	// avisa o meu cliente (j2) q acabou (via redis)
	s.sendToClient(player2Info.ReplyChannel, "Fim_Batalha", req.Resultado)

	color.Green("BATALHA (Peer J2): Fim da batalha enviado ao cliente %s e associação limpa", peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Fim da batalha enviado"})
}

// handlers de troca (p2p entre servers)

// (server 2) o server 1 (host) ta me avisando q uma troca comecou
func (s *Server) handleTradeInitiate(c *gin.Context) {
	var req models.TradeInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha meu player local (j2)
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[req.IdJogadorLocal]
	s.muPlayers.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador local (J2) não encontrado"})
		return
	}

	// guarda no mapa 'tradesPeer' pra gnt saber pra qm responder
	s.muTradesPeer.Lock()
	s.tradesPeer[req.IdTroca] = peerTradeInfo{
		PlayerID: req.IdJogadorLocal,
		HostAPI:  req.HostServidor,
	}
	s.muTradesPeer.Unlock()

	// avisa meu cliente (j2) q a troca comecou
	resp := models.RespostaInicioTroca{
		Mensagem: req.IdOponente, // id do oponente
		IdTroca:  req.IdTroca,
	}
	s.sendToClient(player2Info.ReplyChannel, "Inicio_Troca", resp)

	color.Magenta("TROCA (Peer J2): Troca %s registrada para jogador %s. Host: %s", req.IdTroca, req.IdJogadorLocal, req.HostServidor)
	c.JSON(http.StatusOK, gin.H{"message": "Troca iniciada e registrada"})
}

// (server 1 - host) o server 2 ta me devolvendo a carta q o j2 ofertou
func (s *Server) handleTradeSubmitCard(c *gin.Context) {
	var req models.TradeSubmitCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha a troca q eu to hospedando
	s.muTrades.RLock()
	trade, ok := s.trades[req.IdTroca]
	s.muTrades.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Troca não encontrada (no Host J1)"})
		return
	}

	// joga a carta do j2 no canal q a goroutine 'iniciarTroca' ta esperando
	select {
	case trade.CanalJ2 <- req.Carta:
		color.Magenta("TROCA (Host J1): Recebida carta de J2 para troca %s", req.IdTroca)
		c.JSON(http.StatusOK, gin.H{"message": "Oferta recebida"})
	case <-time.After(5 * time.Second): // timeout
		color.Red("TROCA (Host J1): Timeout ao enviar carta de J2 para canal da troca %s", req.IdTroca)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Timeout interno"})
	}
}

// (server 2) o server 1 (host) ta pedindo a oferta do meu player (j2)
func (s *Server) handleTradeRequestCard(c *gin.Context) {
	var req models.TradeRequestCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// descobre quem eh o meu player (j2)
	s.muTradesPeer.RLock()
	peerInfo, ok := s.tradesPeer[req.IdTroca]
	s.muTradesPeer.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de troca não encontrada (J2)"})
		return
	}

	// acha o canal de resposta dele
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jogador (J2) não encontrado na lista"})
		return
	}

	// manda a msg pro meu cliente (j2) "ei, oferta ai" (via redis)
	resp := models.RespostaPedirCartaTroca{IdTroca: req.IdTroca}
	s.sendToClient(player2Info.ReplyChannel, "Pedir_Carta_Troca", resp)

	color.Magenta("TROCA (Peer J2): Pedido de carta enviado ao cliente %s", peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Pedido de oferta enviado"})
}

// (server 2) o server 1 (host) ta mandando o resultado final da troca
func (s *Server) handleTradeResult(c *gin.Context) {
	var req models.TradeResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// acha o j2 e limpa o mapa
	s.muTradesPeer.Lock()
	peerInfo, ok := s.tradesPeer[req.IdTroca]
	if ok {
		delete(s.tradesPeer, req.IdTroca) // limpeza!
	}
	s.muTradesPeer.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Associação de troca (J2) não encontrada (ou já encerrada)"})
		return
	}

	// acha o canal de resposta do j2
	s.muPlayers.RLock()
	player2Info, ok := s.playerList[peerInfo.PlayerID]
	s.muPlayers.RUnlock()
	if !ok {
		color.Yellow("TROCA (Peer J2): Fim da troca %s, mas J2 (%s) não encontrado. Associação limpa.", req.IdTroca, peerInfo.PlayerID)
		c.JSON(http.StatusOK, gin.H{"message": "Fim da troca processado, mas J2 não encontrado"})
		return
	}

	// avisa o meu cliente (j2) o resultado (via redis)
	// a 'CartaRecebida' aqui eh a carta q o j1 ofertou (e q o j2 ta recebendo)
	resp := models.RespostaResultadoTroca{
		Mensagem:      "Troca concluída!",
		CartaRecebida: req.CartaRecebida,
	}
	s.sendToClient(player2Info.ReplyChannel, "Resultado_Troca", resp)

	color.Magenta("TROCA (Peer J2): Resultado da troca enviado ao cliente %s e associação limpa", peerInfo.PlayerID)
	c.JSON(http.StatusOK, gin.H{"message": "Fim da troca enviado"})
}
