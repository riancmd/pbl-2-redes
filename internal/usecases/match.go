package usecases

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"pbl-2-redes/internal/models"
	"time"
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
		Inbox:            make(chan models.MatchMsg, 0),
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

// Enviar msg
func (u *UseCases) SendMsg(msg models.BattleRequest) {
	select {
	case u.inbox <- msg:
		// entregou

	default:
		slog.Error("inbox cheio")
	}

}

// Dispatcher é a goroutine que ouve o canal principal e envia pras goroutines
func (u *UseCases) Dispatcher() {
	for msg := range u.inbox {
		u.inboxMU.Lock()

		targetInbox, found := u.inboxes[msg.BattleID]

		u.inboxMU.Unlock()

		if found {
			select {
			case targetInbox <- msg.MatchMsg:
				//entregou
			default:
				slog.Error("Inbox cheio")
			}
		}
	}

}

// Goroutine responsável por ouvir se existem batalhas
func (u *UseCases) CheckNewMatches() {
	managedMatches := make(map[string]bool, 0)
	allMatches := []models.Match{}

	// loop de verificação
	for {
		time.Sleep(50 * time.Millisecond)
		allMatches = u.repos.Match.GetAll()

		// se tiver mais de uma partida, passa pela lista
		if u.repos.Match.Length() >= 1 {
			// para cada partida, se for minha partida E não estiver gerenciada, gerencio
			for _, match := range allMatches {
				if match.ServerID != u.sync.GetServerID() {
					continue
				}
				u.matchesMU.Lock()
				// se tiver sendo gerenciada já
				if managedMatches[match.ID] {
					u.matchesMU.Unlock()
					continue
				}
				slog.Error("new match found")
				managedMatches[match.ID] = true

				u.matchesMU.Unlock()

				go u.ManageMatch(match)
			}
		}
	}
}

// Gerencia a partida
func (u *UseCases) ManageMatch(match models.Match) {
	for {
		select {
		// verifica se a mensagem é para minha batalha
		case msg := <-u.inboxes[match.ID]:
			switch msg.Action {
			case "usecard":
				if u.HandleUseCard(msg.Data) {
				}
			case "giveup":
				u.HandleGiveUp(msg.Data)
			}

		case <-timeout:
			u.NotifyBoth(fmt.Sprintf("%s perdeu o turno por timeout", currentPlayer.Username))
		}
	}
}

// HandleUsecard
func (u *UseCases) HandleUseCard(data json.RawMessage) {

}

// HandleGiveUp
func (u *UseCases) HandleGiveUp(data json.RawMessage) {

}

// HandleGiveUp
func (u *UseCases) NotifyBoth(msg, currentPlayerUsername string) {

}

// Atualizar partida
func (u *UseCases) UpdateMatch(match models.Match) error {
	u.sync.UpdateMatch(match)
	return nil
}
