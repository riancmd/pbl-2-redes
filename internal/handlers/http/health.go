package handlers

import (
	"encoding/json"
	"net/http"
)

// Endpoints relacionados ao estoque de cartas, compra e sincronização
func (h Handlers) registerHealthEndpoints() {
	http.HandleFunc("GET /internal/health", h.getHealth)
}

// Retorna todas as cartas do estoque, para sincronização
func (h Handlers) getHealth(w http.ResponseWriter, r *http.Request) {
	health := h.useCases.CheckHealth()
	if health == "alive" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	}
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(health)
}
