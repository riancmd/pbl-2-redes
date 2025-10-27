package bully

import (
	"errors"
	"log/slog"
)

const (
	inElection   int = 0
	postElection int = 1
	leaderless   int = 2
)

// Responsável pela lógica de eleição e sincronização entre servers
type bullyElection struct {
	serverID int   // Por enquanto, é a porta do servidor
	leaderID int   // ID do líder
	peers    []int // IDs dos outros servidores
	state    int   // pode estar em eleição ou pós-eleição
}

func New(serverID int, peers []int) *bullyElection {
	return &bullyElection{
		serverID: serverID,
		leaderID: 0,
		peers:    peers,
		state:    leaderless,
	}
}

// Verifica se é líder
func (b *bullyElection) isLeader() bool {
	if b.leaderID == b.serverID {
		return true
	}
	return false
}

// Modificar líder
func (b *bullyElection) setLeader(newLeader int) error {
	// Verifica se peer realmente existe
	for _, peerID := range b.peers {
		if newLeader == peerID {
			b.leaderID = newLeader
			return nil
		}
	}
	slog.Error("peer is offline")
	return errors.New("peer is offline")
}

// Modificar estado para pós eleição
func (b *bullyElection) endElection() {
	b.state = postElection
}

// Verifica os IDs
func (b *bullyElection) startElection() {
	b.state = inElection
	var leaderID int
	leaderID = b.serverID
	for _, peerID := range b.peers {
		if leaderID < peerID {
			leaderID = peerID
		}
	}

	b.setLeader(leaderID)
	b.endElection()
}

// Torna sem líder
func (b *bullyElection) setLeaderless() {
	b.state = leaderless
}
