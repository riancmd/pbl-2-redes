package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Possui os endpoints internal/matches com todas as partidas
// e endpoints específicos {matchID} para cada partida
func (h Handlers) registerMatchesEndpoints() {
	http.HandleFunc("GET /internal/matches", h.getAllMatches)
	http.HandleFunc("POST /internal/matches/{matchID}", h.addMatch) // Endpoint de matchmake
	http.HandleFunc("PUT /internal/matches/{matchID}", h.updateMatch)
	http.HandleFunc("DELETE /internal/matches/{matchID}", h.deleteMatch)
}

// Retorna todas as partidas, para atualizar
func (h Handlers) getAllMatches(w http.ResponseWriter, r *http.Request) {
	matches := h.useCases.GetAllMatches()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(matches)
}

// Acrescenta nova partida
func (h Handlers) addMatch(w http.ResponseWriter, r *http.Request) {
	var req models.MatchInitialRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na decodificação", Message: err.Error()})

		return
	}

	err := h.useCases.AddMatch(req)

	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na adição de usuário", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Manter ou não?
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
