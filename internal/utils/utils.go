package utils

import (
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/utils/cardDB"
)

type Utils struct {
	CardDB interface {
		InitializeCardsFromJSON(filename string) (map[string]models.Card, error)
		LoadCardsFromFile(filename string) (map[string]models.Card, error)
		CalculateCardCopies(glossary map[string]models.Card, boostersCount int) map[string]int
		CreateCardPool(glossary map[string]models.Card, copies map[string]int) []models.Card
		CreateBoosters(cardPool []models.Card, boostersCount int) ([]models.Booster, error)
	}
}

func New() *Utils {
	return &Utils{
		CardDB: cardDB.New(),
	}
}
