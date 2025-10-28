package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Possui os endpoints internal/matches com todas as partidas
// e endpoints específicos {matchID} para cada partida
func (h Handlers) registerMatchesEndpoints() {
	http.HandleFunc("GET internal/matches", h.getAllMatches)
	http.HandleFunc("PUT internal/matches/{matchID}", h.updateMatch)
	http.HandleFunc("DELETE internal/matches/{matchID}", h.deleteMatch)
}

// Retorna todas as partidas, para atualizar
func (h Handlers) getAllMatches(w http.ResponseWriter, r *http.Request) {
	matches := h.useCases.GetAllMatches()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(matches)
}

// Atualiza uma partida em andamento
func (h Handlers) updateMatch(w http.ResponseWriter, r *http.Request) {
	// ajustar lógica
}

// Finaliza partida
func (h Handlers) deleteMatch(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("matchID")
	err := h.useCases.EndMatch(idStr)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "couldn't end match", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusAccepted) // sucesso
}
