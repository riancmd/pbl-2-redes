package repositories

import (
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/repositories/users"

	"github.com/google/uuid"
)

type Repositories struct {
	User interface {
		GetAll() []models.User
		Add(newUser models.User)
		UserExists(user string) bool
	}
	Card interface {
		GetAll() []models.Booster
		GetBooster(BID int) (models.Booster, error)
		Add(newBooster models.Booster)
		Remove(BID int) error
		Length() int
		CardsEmpty() bool
	}
	Match interface {
		GetAll() []models.Match
		Add(newMatch models.Match)
		MatchExists(ID uuid.UUID) bool
	}
	Queue interface {
		GetAll() []uuid.UUID
		Enqueue(UID uuid.UUID) error
		Dequeue() error
		UserEnqueued(uuid.UUID) bool
	}
}

func New() *Repositories {
	return &Repositories{
		User: users.New(),
	}
}
