package cards

import (
	"errors"
	"pbl-2-redes/internal/models"
)

type Cards struct {
	cards []models.Booster
}

func New() *Cards {
	return &Cards{cards: make([]models.Booster, 0)}
}

func (c Cards) GetAll() []models.Booster {
	return c.cards
}

func (c Cards) CardsEmpty() bool {
	return len(c.cards) == 0
}

func (c *Cards) Add(newBooster models.Booster) {
	c.cards = append(c.cards, newBooster)
}

func (c *Cards) Length() int {
	return len(c.cards)
}

// função recbe o ID do booster (BID)
func (c *Cards) GetBooster(BID int) (models.Booster, error) {
	booster := c.cards[BID]
	return booster, nil
}

// função recbe o ID do booster (BID)
func (c *Cards) Remove(BID int) error {
	// verifica se o vault ta vazio
	if c.CardsEmpty() {
		return errors.New("vault is already empty")
	}
	c.cards = append(c.cards[:BID], c.cards[BID+1:]...)
	return nil
}
