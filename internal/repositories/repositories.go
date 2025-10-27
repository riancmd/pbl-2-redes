package repositories

import (
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/repositories/cards"
	"pbl-2-redes/internal/repositories/matches"
	"pbl-2-redes/internal/repositories/queue"
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
		UserOnMatch(UID uuid.UUID) bool
		MatchEnded(ID uuid.UUID) bool
		Remove(ID uuid.UUID) error
	}
	BattleQueue interface {
		GetAll() []uuid.UUID
		Enqueue(UID uuid.UUID)
		Dequeue() error
		UserEnqueued(uuid.UUID) bool
	}
	TradingQueue interface {
		GetAll() []uuid.UUID
		Enqueue(UID uuid.UUID)
		Dequeue() error
		UserEnqueued(uuid.UUID) bool
	}
}

func New() *Repositories {
	return &Repositories{
		User:         users.New(),
		Card:         cards.New(),
		Match:        matches.New(),
		BattleQueue:  queue.New(),
		TradingQueue: queue.New(),
	}
}
