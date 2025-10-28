package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
	"strconv"
)

// Endpoints relacionados ao estoque de cartas, compra e sincronização
func (h Handlers) registerCardEndpoints() {
	http.HandleFunc("GET /internal/cards", h.getAllCards)
	http.HandleFunc("DELETE /internal/cards/{id}", h.removeBooster)
}

// Retorna todas as cartas do estoque, para sincronização
func (h Handlers) getAllCards(w http.ResponseWriter, r *http.Request) {
	cards := h.useCases.GetAllCards()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cards)
}

// Remove um Booster do estoque a pedido do líder
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
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na remoção de booster", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted) // sucesso
}
