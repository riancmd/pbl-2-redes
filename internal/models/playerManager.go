package models

import (
	"sync"

	"github.com/google/uuid"
)

// gerenciador de jogadores
type PlayerManager struct {
	mu          sync.Mutex
	byUID       map[uuid.UUID]*User
	byUsername  map[string]*User
	activeByUID map[string]*User
}
