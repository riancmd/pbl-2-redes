package handlers

import (
	"/server"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serve como uma camada de adaptação entre as requisições HTTP e a lógica do servidor.
type Handler struct {
	Server *server.Server
}

// NewHandler cria uma nova instância de Handler.
func NewHandler(s *server.Server) *Handler {
	return &Handler{Server: s}
}

// HandlePurchase processa a requisição de compra vinda de um cliente.
func (h *Handler) HandlePurchase(c *gin.Context) {
	var req models.ClientPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo da requisição inválido, falta o clientId"})
		return
	}

	success, wonCard := h.Server.StartPurchaseTransaction(req.ClientID)

	if success {
		c.JSON(http.StatusOK, gin.H{
			"status": "compra bem-sucedida",
			"carta":  wonCard,
		})
	} else {
		c.JSON(http.StatusConflict, gin.H{
			"status": "compra falhou, transação abortada (ex: sem estoque)",
			"carta":  nil,
		})
	}
}

// HandlePreparePurchase processa a fase de preparação do 2PC.
func (h *Handler) HandlePreparePurchase(c *gin.Context) {
	if h.Server.CanPreparePurchase() {
		c.JSON(http.StatusOK, gin.H{"status": "prepared"})
	} else {
		c.JSON(http.StatusConflict, gin.H{"status": "não pode preparar, sem estoque"})
	}
}

// HandleCommitPurchase processa a fase de commit do 2PC.
func (h *Handler) HandleCommitPurchase(c *gin.Context) {
	h.Server.CommitPurchase()
	c.JSON(http.StatusOK, gin.H{"status": "committed"})
}

// HandleAbortPurchase processa a fase de abort do 2PC.
func (h *Handler) HandleAbortPurchase(c *gin.Context) {
	log.Printf("[%s] Recebido ABORT.", h.Server.MyAddress)
	c.JSON(http.StatusOK, gin.H{"status": "aborted"})
}
