package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"

	"github.com/google/uuid"
)

// Retorna repositório contendo todas as partidas
// Serve para sincronizar as partidas
func (u UseCases) GetAllMatches() []models.Match {
	matches := u.repos.Match.GetAll()
	return matches
}

// Adiciona uma partida à lista de partidas
func (u UseCases) AddMatch(P1, P2 models.User) error {
	// Verifica se usuário já está em partida
	players := make([]models.User, 0)
	players = append(players, P1, P2)

	for _, player := range players {
		onMatch := u.repos.Match.UserOnMatch(player.UID)
		if onMatch {
			slog.Error("this user is already playing", "username", player.Username)
			return errors.New("user already on a match")
		}
	}

	repoReq := models.Match{
		ID:    uuid.New().String(),
		P1:    &P1,
		P2:    &P2,
		State: models.Running,
		Turn:  P1.UID,

		Hand:             map[string][]*models.Card{},
		Sanity:           map[string]int{P1.UID: 40, P2.UID: 40},
		DreamStates:      map[string]models.DreamState{P1.UID: models.Sleepy, P2.UID: models.Sleepy},
		RoundsInState:    map[string]int{P1.UID: 0, P2.UID: 0},
		StateLockedUntil: map[string]int{P1.UID: 0, P2.UID: 0},
		CurrentRound:     1,
		//inbox:            make(chan models.matchMsg, 16),
	}

	err := u.sync.MatchNew(repoReq)

	if err != nil {
		slog.Error("couldn't create match")
		return err
	}

	u.repos.Match.Add(repoReq)

	return nil
}

// Finaliza partida
func (u UseCases) EndMatch(ID string) error {
	// Verifica se partida realmente finalizou
	finished := u.repos.Match.MatchEnded(ID)

	if !finished {
		slog.Error("this battle hasn't finished yet", "battleID", ID)
		return errors.New("battle is still going")
	}

	u.sync.MatchEnd(ID)
	err := u.repos.Match.Remove(ID)

	if err != nil {
		return err
	}

	return nil
}
