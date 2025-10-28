package models

import (
	"time"

	"github.com/google/uuid"
)

// registro do usuário em memória
type User struct {
	UID         string
	Username    string
	Password    string
	Deck        []*Card
	CreatedAt   time.Time
	LastLogin   time.Time
	TotalWins   int
	TotalLosses int
	IsInBattle  bool
}

type CreateUserRequest struct {
	UID      uuid.UUID
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateUserResponse struct {
	NewUserID   uuid.UUID `json:"newUserID"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	Deck        []*Card   `json:"cards"`
	CreatedAt   time.Time `json:"created_at"`
	LastLogin   time.Time `json:"last_login"`
	TotalWins   int       `json:"total_wins"`
	TotalLosses int       `json:"total_losses"`
	IsInBattle  bool      `json:"isInBattle"`
}
