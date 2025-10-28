package models

import (
	"encoding/json"
	"sync"
)

// SISTEMA DE MATCHMAKING
// mensagem interna de jogo para a goroutine do Match
type matchMsg struct {
	PlayerUID string
	Action    string
	Data      json.RawMessage
}

type MatchState int

const (
	Waiting MatchState = iota
	Running
	Finished
)

type Match struct {
	ID     string
	P1, P2 *User
	State  MatchState
	Turn   string // ID do jogador que joga a próxima ação

	Hand             map[string][]*Card // 10 cartas por jogador
	Sanity           map[string]int     // pontos por jogador
	DreamStates      map[string]DreamState
	RoundsInState    map[string]int // para controlar duração dos estados
	StateLockedUntil map[string]int // para controlar quando pode mudar estado
	CurrentRound     int

	//inbox chan matchMsg // canal para trocar msgs entre threads
	//mu *sync.Mutex
}

type MatchManager struct {
	mu       sync.Mutex
	queue    []*User
	nextID   int
	matches  map[int]*Match
	byPlayer map[string]*Match
}

// SISTEMA DE BATALHAS
type BattleState int

const (
	WaitingForPlayers BattleState = iota
	InProgress
	Completed
)
