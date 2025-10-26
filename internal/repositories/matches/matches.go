package matches

import (
	"pbl-2-redes/internal/models"

	"github.com/google/uuid"
)

type Matches struct {
	matches []models.Match
}

func New() *Matches {
	return &Matches{matches: make([]models.Match, 0)}
}

func (m Matches) GetAll() []models.Match {
	return m.matches
}

func (m Matches) MatchExists(ID uuid.UUID) bool {
	for _, v := range m.matches {
		if v.ID == ID {
			return true
		}
	}
	return false
}

func (m *Matches) Add(newMatch models.Match) {
	m.matches = append(m.matches, newMatch)
}
