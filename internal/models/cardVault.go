package models

import (
	"math/rand"
)

// BANCO DE CARTAS
type CardVault struct {
	CardGlossary map[string]Card
	CardQuantity map[string]int

	Vault           map[int]Booster
	BoosterQuantity int
	Total           int
	Generator       *rand.Rand
}

// struct pra base de dados local das cartas em json porem virtualizada
type CardDatabase struct {
	Cards map[string]Card `json:"cards"`
}
