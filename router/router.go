package router

import (
	"card-game/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRouter configura e retorna o roteador Gin com todas as rotas da aplicação.
func SetupRouter(h *handlers.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Rota pública para clientes iniciarem uma compra.
	router.POST("/purchase", h.HandlePurchase)

	// Rotas internas para a comunicação do 2-Phase Commit entre os servidores.
	router.POST("/prepare-purchase", h.HandlePreparePurchase)
	router.POST("/commit-purchase", h.HandleCommitPurchase)
	router.POST("/abort-purchase", h.HandleAbortPurchase)

	return router
}
