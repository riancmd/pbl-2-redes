package models

// struct pra base de dados local das cartas em json porem virtualizada
type CardDB struct {
	Cards map[string]Card `json:"cards"`
}
