package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Trabalha com os endpoints relacionadas a fila de troca
func (h Handlers) registerTradingQueueEndpoints() {
	http.HandleFunc("GET internal/trading_queue", h.getTradingQueue)
	http.HandleFunc("POST internal/trading_queue", h.tradingEnqueue)
	http.HandleFunc("DELETE internal/trading_queue", h.tradingDequeue)
}

// Retorna a fila inteira
func (h Handlers) getTradingQueue(w http.ResponseWriter, r *http.Request) {
	queue := h.useCases.Trading_GetAllEnqueuedPlayers()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(queue)
}

// Adiciona à fila
func (h Handlers) tradingEnqueue(w http.ResponseWriter, r *http.Request) {
	var req string

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na fila de troca", Message: err.Error()})

		return
	}

	err := h.useCases.Trading_Enqueue(req)

	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na fila de troca", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Remove da fila
func (h Handlers) tradingDequeue(w http.ResponseWriter, r *http.Request) {
	err := h.useCases.Trading_Dequeue()

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na saída da fila de troca", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted) // sucesso
}
