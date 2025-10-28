package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Trabalha com os endpoints relacionadas a fila de batalha
func (h Handlers) registerBattleQueueEndpoints() {
	http.HandleFunc("GET internal/battle_queue", h.getBattleQueue)
	http.HandleFunc("POST internal/battle_queue", h.battleEnqueue)
	http.HandleFunc("DELETE internal/battle_queue", h.battleDequeue)
}

// Retorna toda a fila
func (h Handlers) getBattleQueue(w http.ResponseWriter, r *http.Request) {
	queue := h.useCases.Battle_GetAllEnqueuedPlayers()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(queue)
}

// Acrescenta usuário à fila
func (h Handlers) battleEnqueue(w http.ResponseWriter, r *http.Request) {
	var req string

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na fila de batalha", Message: err.Error()})

		return
	}

	err := h.useCases.Battle_Enqueue(req)

	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na fila de batalha", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Dá um pop na fila
func (h Handlers) battleDequeue(w http.ResponseWriter, r *http.Request) {
	err := h.useCases.Battle_Dequeue()

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na Saída da Fila de batalha", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted) // sucesso
}
