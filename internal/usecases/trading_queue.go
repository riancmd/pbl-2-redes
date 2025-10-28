package usecases

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
)

// Retorna a fila inteira de trocas
func (u UseCases) Trading_GetAllEnqueuedPlayers() []string {
	queue := u.repos.BattleQueue.GetAll()
	return queue
}

// Coloca na filha de trocas
func (u UseCases) Trading_Enqueue(user models.User) error {
	enqueued := u.repos.BattleQueue.UserEnqueued(user.UID)

	if enqueued {
		slog.Error("this user is already enqueued", "username", user.Username)
		return errors.New("user is already enqueued")
	}

	// Sincroniza entre servidores
	err := u.sync.BattleEnqueue(user.UID)

	if err != nil {
		slog.Error("this user is already enqueued", "username", user.Username)
		return err
	}

	u.repos.BattleQueue.Enqueue(user.UID)

	return nil
}

// Dá um pop na fila
func (u UseCases) Trading_Dequeue() error {
	// Sincroniza entre servidores
	err := u.sync.BattleDequeue()

	if err != nil {
		slog.Error("couldn't dequeue player")
		return err
	}

	// se pôde fazer o dequeue, continua
	empty := u.repos.BattleQueue.Dequeue()

	if empty != nil {
		slog.Error("queue is already empty")
		return empty
	}

	return nil
}
