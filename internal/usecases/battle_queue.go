package usecases

import (
	"errors"
	"log/slog"
)

// Retorna a fila inteira de batalha
func (u UseCases) Battle_GetAllEnqueuedPlayers() []string {
	queue := u.repos.BattleQueue.GetAll()
	return queue
}

// Coloca na filha de batalha
func (u UseCases) Battle_Enqueue(UID string) error {
	enqueued := u.repos.BattleQueue.UserEnqueued(UID)

	if enqueued {
		slog.Error("this user is already enqueued")
		return errors.New("user is already enqueued")
	}

	// Sincroniza entre servidores
	err := u.sync.BattleEnqueue(UID)

	if err != nil {
		slog.Error("this user is already enqueued")
		return err
	}

	u.repos.BattleQueue.Enqueue(UID)

	return nil
}

// Dá um pop na fila
func (u UseCases) Battle_Dequeue() error {
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
