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

			matchReq := models.MatchInitialRequest{
				ID:       (uuid.New()).String(),
				ServerID: u.sync.GetServerID(),
				P1:       p1,
				P2:       p2,
			}

			u.AddMatch(matchReq)
		}
	}
}
