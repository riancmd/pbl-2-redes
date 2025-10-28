package queue

import (
	"errors"
)

type Queue struct {
	queue []string
}

func New() *Queue {
	return &Queue{queue: make([]string, 0)}
}

func (q Queue) GetAll() []string {
	return q.queue
}

func (q Queue) UserEnqueued(UID string) bool {
	for _, v := range q.queue {
		if v == UID {
			return true
		}
	}
	return false
}

func (q *Queue) Enqueue(UID string) {
	q.queue = append(q.queue, UID)
}

func (q *Queue) Dequeue() error {
	if len(q.queue) >= 1 {
		q.queue = q.queue[1:]
		return nil
	}
	return errors.New("queue is empty")
}
