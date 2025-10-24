package utils

import (
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/utils/cardDB"
)

type Utils struct {
	CardDB interface {
		InitializeCardsFromJSON(filename string) (map[string]models.Card, error)
		LoadCardsFromFile(filename string) error
		CalculateCardCopies(boostersCount int) map[string]int
		CreateCardPool(copies map[string]int) []models.Card
		CreateBoosters(boostersCount int) error
	}
}

func New() *Utils {
	return &Utils{
		CardDB: cardDB.New(),
	}
}
