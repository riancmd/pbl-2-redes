package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
	"time"

	"github.com/google/uuid"
)

func (u UseCases) GetAllUsers() []models.User {
	users := u.repos.User.GetAll()
	return users
}

func (u UseCases) AddUser(newUser models.CreateUserRequest) (uuid.UUID, error) {
	exists := u.repos.User.UserExists(newUser.Username)

	if exists {
		slog.Error("this user already exists", "username", newUser.Username)
		return uuid.Nil, errors.New("user already exists")
	}

	repoReq := models.User{
		UID:         uuid.New(),
		Username:    newUser.Username,
		Password:    newUser.Password,
		Deck:        make([]*models.Card, 0),
		CreatedAt:   time.Now(),
		LastLogin:   time.Now(),
		TotalWins:   0,
		TotalLosses: 0,
		IsInBattle:  false,
	}

	u.repos.User.Add(repoReq)

	return repoReq.UID, nil
}
