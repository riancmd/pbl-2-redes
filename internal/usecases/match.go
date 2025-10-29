package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
)

// Retorna repositório contendo todas as partidas
// Serve para sincronizar as partidas
func (u *UseCases) GetAllMatches() []models.Match {
	matches := u.repos.Match.GetAll()
	return matches
}

// Adiciona uma partida à lista de partidas
func (u *UseCases) AddMatch(matchReq models.MatchInitialRequest) error {
	if !u.sync.IsLeader() {
		// Verifica se usuário já está em partida
		players := make([]string, 0)
		players = append(players, matchReq.P1, matchReq.P2)

		for _, player := range players {
			onMatch := u.repos.Match.UserOnMatch(player)
			if onMatch {
				slog.Error("user is already on a match")
				return errors.New("user already on a match")
			}
		}
		err := u.sync.MatchNew(matchReq)

		if err != nil {
			slog.Error("couldn't create match")
			return err
		}

		return nil
	}

	handP1, err := u.sync.GetHand(matchReq.P1)
	if err != nil {
		return err
	}
	handP2, err := u.sync.GetHand(matchReq.P1)
	if err != nil {
		return err
	}

	// estrutura request da partida
	mReq := models.Match{
		ID:    matchReq.ID,
		P1:    matchReq.P1,
		P2:    matchReq.P2,
		State: models.Running,
		Turn:  matchReq.P1,

		Hand:             map[string][]*models.Card{},
		Sanity:           map[string]int{matchReq.P1: 40, matchReq.P2: 40},
		DreamStates:      map[string]models.DreamState{matchReq.P2: models.Sleepy, matchReq.P2: models.Sleepy},
		RoundsInState:    map[string]int{matchReq.P1: 0, matchReq.P2: 0},
		StateLockedUntil: map[string]int{matchReq.P1: 0, matchReq.P2: 0},
		CurrentRound:     1,
		inbox:            make(chan models.MatchMsg, 16),
	}

	mReq.Hand[mReq.P1] = handP1
	mReq.Hand[mReq.P2] = handP2

	u.matchesMU.Lock()

	u.repos.Match.Add(mReq)

	u.matchesMU.Unlock()

	return nil
}

// Finaliza partida
func (u *UseCases) EndMatch(ID string) error {
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

// Atualizar partida
func (u *UseCases) UpdateMatch(match models.Match) error {
	u.sync.UpdateMatch(match)
	return nil
}
