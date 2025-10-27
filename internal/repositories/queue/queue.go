package queue

import (
	"errors"

	"github.com/google/uuid"
)

type Queue struct {
	queue []uuid.UUID
}

func New() *Queue {
	return &Queue{queue: make([]uuid.UUID, 0)}
}

func (q Queue) GetAll() []uuid.UUID {
	return q.queue
}

func (q Queue) UserEnqueued(UID uuid.UUID) bool {
	for _, v := range q.queue {
		if v == UID {
			return true
		}
	}
	return false
}

func (q *Queue) Enqueue(UID uuid.UUID) {
	q.queue = append(q.queue, UID)
}

func (q *Queue) Dequeue() error {
	if len(q.queue) >= 1 {
		q.queue = q.queue[1:]
		return nil
	}
	return errors.New("queue is empty")
}
