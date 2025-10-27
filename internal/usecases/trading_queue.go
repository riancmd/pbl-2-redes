package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"

	"github.com/google/uuid"
)

func (u UseCases) Trading_GetAllEnqueuedPlayers() []uuid.UUID {
	queue := u.repos.BattleQueue.GetAll()
	return queue
}

func (u UseCases) Trading_Enqueue(user models.User) error {
	enqueued := u.repos.BattleQueue.UserEnqueued(user.UID)

	if enqueued {
		slog.Error("this user is already enqueued", "username", user.Username)
		return errors.New("user is already enqueued")
	}

	u.repos.BattleQueue.Enqueue(user.UID)

	return nil
}

func (u UseCases) Trading_Dequeue() error {
	empty := u.repos.BattleQueue.Dequeue()

	if empty != nil {
		slog.Error("queue is already empty")
		return empty
	}

	return nil
}
