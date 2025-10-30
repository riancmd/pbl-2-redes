package main

import (
	"github.com/gin-gonic/gin"
)

// setupRouter inicializa e configura o motor Gin.
func (s *Server) setupRouter() *gin.Engine {
	// gin.SetMode(gin.ReleaseMode) // Descomente para produção
	r := gin.Default()

	// Rota para eleição de líder e verificação de saúde
	r.GET("/health", s.handleHealthCheck)

	// #################################################
	// # Rotas de Sincronização (Líder e Seguidores)
	// #################################################
	
	// Rotas para gerenciamento de jogadores
	playerGroup := r.Group("/players")
	{
		// Seguidor -> Líder: Notifica o líder sobre um novo jogador
		playerGroup.POST("/connect", s.handleLeaderConnect)
		
		// Líder -> Seguidor: Notifica seguidores sobre a lista atualizada
		playerGroup.POST("/update", s.handlePlayerUpdate)
	}

	// Rotas para gerenciamento de cartas (compra)
	cardGroup := r.Group("/cards")
	{
		// Seguidor -> Líder: Pede ao líder para processar uma compra
		cardGroup.POST("/buy", s.handleLeaderBuyCard)
	}

	// Rotas para gerenciamento de inventário
	inventoryGroup := r.Group("/inventory")
	{
		// Líder -> Seguidor: Notifica sobre mudança no estoque
		inventoryGroup.POST("/update", s.handleInventoryUpdate)
	}

	// #################################################
	// # Rotas de Batalha (Comunicação P2P entre Servidores)
	// #################################################
	
	// Estas rotas são usadas para a comunicação entre S1 (Host) e S2 (Peer)
	battleGroup := r.Group("/battle")
	{
		// S1 (Host) -> S2 (Peer): Inicia uma batalha
		battleGroup.POST("/initiate", s.handleBattleInitiate)
		
		// S1 (Host) -> S2 (Peer): Pede a jogada do J2
		battleGroup.POST("/request_move", s.handleBattleRequestMove)
		
		// S1 (Host) -> S2 (Peer): Informa o resultado do turno
		battleGroup.POST("/turn_result", s.handleBattleTurnResult)
		
		// S1 (Host) -> S2 (Peer): Encerra a batalha
		battleGroup.POST("/end", s.handleBattleEnd)
		
		// S2 (Peer) -> S1 (Host): Envia a carta/jogada do J2
		battleGroup.POST("/submit_move", s.handleBattleSubmitMove)
	}

	return r
}