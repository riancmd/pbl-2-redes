package matches

import (
	"errors"
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

func (m Matches) UserOnMatch(UID uuid.UUID) bool {
	for _, v := range m.matches {
		if v.P1.UID == UID {
			return true
		}
	}
	return false
}

func (m *Matches) Add(newMatch models.Match) {
	m.matches = append(m.matches, newMatch)
}

func (m Matches) MatchEnded(ID uuid.UUID) bool {
	for _, v := range m.matches {
		if v.ID == ID && v.State == models.Finished {
			return true
		}
	}
	return false
}

func (m Matches) Remove(matchID uuid.UUID) error {
	for index, match := range m.matches {
		if match.ID == matchID {
			m.matches = append(m.matches[:index], m.matches[(index+1):]...)
			return nil
		}
	}
	return errors.New("match has already ended")
}
