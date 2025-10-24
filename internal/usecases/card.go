package usecases

import (
	"errors"
	"log/slog"
	"math/rand"
	"pbl-2-redes/internal/models"
	"time"
)

func (u UseCases) GetAllCards() []models.User {
	users := u.repos.User.GetAll()
	return users
}

func (u UseCases) AddCards(newBooster models.Booster) error {
	if u.repos.Card == nil {
		return errors.New("vault doesn't exist")
	}

	u.repos.Card.Add(newBooster)

	return nil
}

func (u UseCases) GetBooster() (models.Booster, error) {
	// verifica se vault vazio
	empty := u.repos.Card.CardsEmpty()

	if empty {
		slog.Error("vault is empty")
		return models.Booster{}, nil
	}

	// pega um indice aleatorio
	generator := rand.New(rand.NewSource(time.Now().UnixNano())) // gerador
	randomIndex := generator.Intn(u.repos.Card.Length())
	return u.repos.Card.GetBooster(randomIndex)
}

func (u UseCases) RemoveBooster(BID int) error {
	return u.repos.Card.Remove(BID)
}

func (u UseCases) AddCardsFromFile(filename string) error {

}
