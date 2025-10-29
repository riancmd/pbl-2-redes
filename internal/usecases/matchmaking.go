package usecases

import (
	"pbl-2-redes/internal/models"
	"time"

	"github.com/google/uuid"
)

// Go routine que vai ficar verificando se pode criar partidas
func (u *UseCases) matchmakingLoop() {
	for {
		time.Sleep(50 * time.Millisecond)
		if u.repos.BattleQueue.Length() >= 2 {
			p1, p2 := u.repos.BattleQueue.GetDequeuedPlayers()

			if !u.UIDExists(p1) && !u.UIDExists(p2) {
				continue
			}

			Server1 := u.sync.FindServer(p1)
			Server2 := u.sync.FindServer(p2)

			if u.UIDExists(p1) {
				Server1 = u.sync.GetServerID()
			}

			if u.UIDExists(p2) {
				Server2 = u.sync.GetServerID()
			}

			matchReq := models.MatchInitialRequest{
				ID:      (uuid.New()).String(),
				Server1: Server1,
				Server2: Server2,
				P1:      p1,
				P2:      p2,
			}

			u.AddMatch(matchReq)
		}
	}
}
