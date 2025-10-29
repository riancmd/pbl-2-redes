package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Endpoints relacionados à lista de usuários individual de cada server
func (h Handlers) registerUserEndpoints() {
	http.HandleFunc("GET /internal/users", h.getAllUsers)
	http.HandleFunc("POST /internal/users", h.addUser)
	http.HandleFunc("GET /internal/users{username}", h.userExists)
	http.HandleFunc("GET /internal/users{uid}", h.uidExists)
	http.HandleFunc("GET /internal/users/{uid}/hand", h.getHand)
}

// Retorna todos os usuários (possivelmente não utilizada)
func (h Handlers) getAllUsers(w http.ResponseWriter, r *http.Request) {
	users := h.useCases.GetAllUsers()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}

// Acrescenta usuário
func (h Handlers) addUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na adição de usuário", Message: err.Error()})

		return
	}

	err := h.useCases.AddUser(req)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "Erro na adição de usuário", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Retorna se usuário existe ou não
func (h Handlers) userExists(w http.ResponseWriter, r *http.Request) {
	userStr := r.PathValue("username")
	exists := h.useCases.UserExists(userStr)
	if exists {
		w.WriteHeader(http.StatusFound)

		return
	}

	w.WriteHeader(http.StatusNotFound)
}

// Retorna se usuário existe ou não
func (h Handlers) uidExists(w http.ResponseWriter, r *http.Request) {
	userStr := r.PathValue("uid")
	exists := h.useCases.UIDExists(userStr)
	if exists {
		w.WriteHeader(http.StatusFound)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

// Retorna mão
func (h Handlers) getHand(w http.ResponseWriter, r *http.Request) {
	userStr := r.PathValue("uid")
	exists := h.useCases.UserExists(userStr)

	// se encontrou usuário
	if exists {
		hand, err := h.useCases.GetHand(userStr)

		if err != nil {
			w.WriteHeader(http.StatusForbidden) // poucas cartas
			json.NewEncoder(w).Encode(hand)
			return
		}

		// status sucesso, conseguiu criar mão
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(hand)
	} else {
		// se não encontrou usuário
		w.WriteHeader(http.StatusNotFound)
		return
	}

}
