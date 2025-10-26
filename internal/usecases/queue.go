package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"

	"github.com/google/uuid"
)

func (u UseCases) GetAllEnqueuedPlayers() []uuid.UUID {
	queue := u.repos.Queue.GetAll()
	return queue
}

func (u UseCases) Enqueue(user models.User) error {
	enqueued := u.repos.Queue.UserEnqueued(user.UID)

	if enqueued {
		slog.Error("this user is already enqueued", "username", user.Username)
		return errors.New("user is already enqueued")
	}

	u.repos.Queue.Enqueue(user.UID)

	return nil
}

func (u UseCases) Dequeue() error {
	empty := u.repos.Queue.Dequeue()

	if empty != nil {
		slog.Error("queue is already empty")
		return empty
	}

	return nil
}
