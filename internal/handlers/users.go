package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

func (h Handlers) registerUserEndpoints() {
	http.HandleFunc("GET /users", h.getAllUsers)
	http.HandleFunc("POST /users", h.addUser)
}

func (h Handlers) getAllUsers(w http.ResponseWriter, r *http.Request) {
	users := h.useCases.GetAllUsers()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}

func (h Handlers) addUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "UserHandlerErr", Message: err.Error()})

		return
	}

	id, err := h.useCases.AddUser(req)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Type: "UserHandlerErr", Message: err.Error()})

		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.CreateUserResponse{NewUserID: id})
}
