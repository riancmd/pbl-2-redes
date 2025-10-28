package usecases

import (
	"pbl-2-redes/internal/models"
)

// PORTS define interfaces para conexão entre usecases e o cluster
type ClusterSync interface {
	// Sincroniza o estoque de cartas
	SyncCards() ([]models.Booster, error)
	// Sincroniza o enqueue na fila de batalha
	BattleEnqueue(UID string) error
	// Sincroniza o dequeue na fila de batalha
	BattleDequeue() error
	// Sincroniza o enqueue na fila de troca
	TradingEnqueue(UID string) error
	// Sincroniza o dequeue na fila de troca
	TradingDequeue() error
	// Sincroniza nova batalha
	MatchNew(Match models.Match) error
	// Sincroniza nova batalha
	MatchEnd(string) error
	// Sincroniza compra de carta
	BuyBooster(boosterID int) error
	// Sincroniza troca de carta
	TradeCard() error
	// Sincroniza criação de usuários, para não permitir cópias
	UserNew(username string) error
	//..........
}
