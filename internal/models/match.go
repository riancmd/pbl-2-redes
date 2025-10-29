package models

import (
	"encoding/json"
	"sync"
)

// SISTEMA DE MATCHMAKING
// mensagem interna de jogo para a goroutine do Match
type MatchMsg struct {
	PlayerUID string
	Action    string
	Data      json.RawMessage
}

// mensagem que PUBSUB envia no canal do usecases
type BattleRequest struct {
	BattleID string
	MatchMsg MatchMsg
}

type MatchState int

const (
	Waiting MatchState = iota
	Running
	Finished
)

type Match struct {
	ID       string
	ServerID int    // port
	P1, P2   string // UID
	State    MatchState
	Turn     string // ID do jogador que joga a próxima ação

	Hand             map[string][]*Card // 10 cartas por jogador
	Sanity           map[string]int     // pontos por jogador
	DreamStates      map[string]DreamState
	RoundsInState    map[string]int // para controlar duração dos estados
	StateLockedUntil map[string]int // para controlar quando pode mudar estado
	CurrentRound     int

	Inbox chan MatchMsg // canal para trocar msgs entre threads
	//mu *sync.Mutex
}

type MatchInitialRequest struct {
	ID       string
	ServerID int    // port
	P1, P2   string // UID
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
