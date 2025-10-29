package repositories

import (
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/repositories/cards"
	"pbl-2-redes/internal/repositories/matches"
	"pbl-2-redes/internal/repositories/queue"
	"pbl-2-redes/internal/repositories/users"
)

type Repositories struct {
	User interface {
		GetAll() []models.User
		Add(newUser models.User)
		UserExists(user string) bool
		UIDExists(uid string) bool
		CheckPassword(usern string, password string) (bool, error)
		SwitchCard(UID, CID string, card models.Card) error
		GetDeck(UID string) []*models.Card
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
		MatchExists(ID string) bool
		UserOnMatch(UID string) bool
		MatchEnded(ID string) bool
		Remove(ID string) error
	}
	BattleQueue interface {
		GetAll() []string
		Enqueue(UID string)
		Dequeue() error
		UserEnqueued(string) bool
	}
	TradingQueue interface {
		GetAll() []string
		Enqueue(UID string)
		Dequeue() error
		UserEnqueued(string) bool
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
