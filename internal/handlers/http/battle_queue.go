package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

func (h Handlers) registerBattleQueueEndpoints() {
	http.HandleFunc("GET /battlequeue", h.getBattleQueue)
	http.HandleFunc("POST /battlequeue", h.battleEnqueue)
	http.HandleFunc("DELETE /battlequeue", h.battleDequeue)
}

func (h Handlers) getBattleQueue(w http.ResponseWriter, r *http.Request) {
	queue := h.useCases.Battle_GetAllEnqueuedPlayers()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(queue)
}

func (h Handlers) battleEnqueue(w http.ResponseWriter, r *http.Request) {
	var req models.User

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Reason: err.Error()})

		return
	}

	err := h.useCases.Battle_Enqueue(req)

	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Reason: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h Handlers) battleDequeue(w http.ResponseWriter, r *http.Request) {
	err := h.useCases.Battle_Dequeue()

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Reason: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted) // sucesso
}
