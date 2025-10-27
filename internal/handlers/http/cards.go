package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
	"strconv"
)

func (h Handlers) registerCardEndpoints() {
	http.HandleFunc("GET /cards", h.getAllCards)
	http.HandleFunc("DELETE /cards/{id}", h.removeBooster)
}

func (h Handlers) getAllCards(w http.ResponseWriter, r *http.Request) {
	cards := h.useCases.GetAllCards()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cards)
}

func (h Handlers) removeBooster(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	// converte para string
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	err = h.useCases.RemoveBooster(id)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Reason: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted) // sucesso
}
