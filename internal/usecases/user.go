package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
	"time"
)

// Retorna lista com todos os usuários
func (u UseCases) GetAllUsers() []models.User {
	users := u.repos.User.GetAll()
	return users
}

// Acrescenta novo usuário
func (u UseCases) AddUser(newUser models.CreateUserRequest) error {
	exists := u.repos.User.UserExists(newUser.Username)

	if exists {
		slog.Error("this user already exists", "username", newUser.Username)
		return errors.New("user already exists")
	}

	repoReq := models.User{
		UID:         newUser.UID,
		Username:    newUser.Username,
		Password:    newUser.Password,
		Deck:        make([]*models.Card, 0),
		CreatedAt:   time.Now(),
		LastLogin:   time.Now(),
		TotalWins:   0,
		TotalLosses: 0,
		IsInBattle:  false,
	}

	// Verifica se existe em outro servidor
	err := u.sync.UserNew(newUser.Username)

	if err != nil {
		slog.Error("this user already exists", "username", newUser.Username)
		return err
	}

	u.repos.User.Add(repoReq)

	return nil
}

// Verifica se usuário existe
func (u UseCases) UserExists(username string) bool {
	exists := u.repos.User.UserExists(username)
	return exists
}
