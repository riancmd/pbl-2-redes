package main

import (
	"PlanoZ/models"

	"github.com/fatih/color"
)

// limparRecursosServidoresMortos é a função principal chamada pelo loop de health check
func (s *Server) limparRecursosServidoresMortos(deadServerIDs []string) {
	color.Magenta("[Limpeza]: Iniciando limpeza para servidores mortos: %v", deadServerIDs)

	// Converte IDs (ex: "server1") para Hosts API (ex: "server1:9090")
	deadServerHosts := make(map[string]bool)
	for _, id := range deadServerIDs {
		if host, ok := s.serverList[id]; ok {
			deadServerHosts[host] = true
		}
	}

	// Executa as limpezas em goroutines separadas
	go s.limparBatalhasHost(deadServerIDs)
	go s.limparBatalhasPeer(deadServerHosts)
	go s.limparPlayers(deadServerIDs)
}

// limparBatalhasHost (Cenário: Este servidor é o HOST, o Peer/J2 Morreu)
// Este servidor é o HOST (S1). Verificamos se algum J2 estava em um servidor (S2) que morreu.
func (s *Server) limparBatalhasHost(deadServerIDs []string) {
	batalhasAMatar := make(map[string]string) // map[battleID] -> vencedor (J1)

	s.muBatalhas.RLock()
	s.muPlayers.RLock()

	for battleID, batalha := range s.batalhas {
		// Encontra em qual servidor o J2 (oponente) estava
		infoJ2, ok := s.playerList[batalha.Jogador2]
		if !ok {
			continue // Jogador já deve ter sido limpo
		}

		for _, deadID := range deadServerIDs {
			if infoJ2.ServerID == deadID {
				// O servidor do J2 morreu. J1 (nosso cliente) vence.
				batalhasAMatar[battleID] = batalha.Jogador1
				break
			}
		}
	}

	s.muPlayers.RUnlock()
	s.muBatalhas.RUnlock()

	// Agora, fora dos locks, encerra as batalhas
	for battleID, vencedor := range batalhasAMatar {
		color.Red("[Limpeza Host]: Encerrando batalha %s. Servidor do J2 (%s) caiu.", battleID, vencedor)
		// Isso notificará nosso cliente J1
		s.encerrarBatalha(battleID, vencedor, "Servidor do oponente caiu")
	}
}

// limparBatalhasPeer (Cenário: Este servidor é o PEER, o Host/J1 Morreu)
// Este servidor é o PEER (S2). Verificamos se o HOST (S1) de alguma batalha morreu.
func (s *Server) limparBatalhasPeer(deadServerHosts map[string]bool) {
	batalhasAMatar := make(map[string]peerBattleInfo) // map[battleID] -> peerInfo

	s.muBatalhasPeer.Lock() // Lock total para ler e deletar
	for battleID, peerInfo := range s.batalhasPeer {
		if deadServerHosts[peerInfo.HostAPI] {
			// O servidor Host (S1) desta batalha morreu.
			batalhasAMatar[battleID] = peerInfo
			delete(s.batalhasPeer, battleID) // Remove do mapa
		}
	}
	s.muBatalhasPeer.Unlock()

	// Agora, notifica os clientes J2 locais
	s.muPlayers.RLock()
	defer s.muPlayers.RUnlock()

	for battleID, peerInfo := range batalhasAMatar {
		color.Red("[Limpeza Peer]: Encerrando batalha %s. Servidor Host (%s) caiu.", battleID, peerInfo.HostAPI)

		infoJ2, ok := s.playerList[peerInfo.PlayerID]
		if ok {
			// Notifica o nosso cliente (J2) que a batalha acabou
			s.sendToClient(infoJ2.ReplyChannel, "Fim_Batalha", models.RespostaFimBatalha{
				Mensagem: "Batalha encerrada! O servidor que hospedava a partida caiu.",
			})
		}
	}
}

// limparPlayers remove todos os jogadores da lista global que estavam conectados aos servidores mortos
func (s *Server) limparPlayers(deadServerIDs []string) {
	// Apenas o LÍDER deve limpar a lista de jogadores
	if !s.isLeader() {
		return
	}

	color.Magenta("[Limpeza Players]: Líder limpando jogadores de servidores mortos...")

	playersARemover := []string{}
	s.muPlayers.Lock()
	for playerID, info := range s.playerList {
		for _, deadID := range deadServerIDs {
			if info.ServerID == deadID {
				playersARemover = append(playersARemover, playerID)
				break
			}
		}
	}

	// Remove os jogadores da lista local
	removedInfo := make(map[string]PlayerInfo) // Guarda info para notificar
	for _, playerID := range playersARemover {
		if info, ok := s.playerList[playerID]; ok {
			removedInfo[playerID] = info
			delete(s.playerList, playerID)
		}
	}
	s.muPlayers.Unlock()

	// Transmite a remoção para os outros servidores VIVOS
	for playerID, info := range removedInfo {
		updateRemove := models.UpdatePlayerListRequest{
			PlayerID: playerID,
			ServerID: info.ServerID, // O ID do servidor MORTO
			Acao:     "remove",
		}
		s.broadcastToServers("/players/update", updateRemove)
	}

	if len(playersARemover) > 0 {
		color.Magenta("[Limpeza Players]: Líder removeu %d jogadores e notificou seguidores.", len(playersARemover))
	}
}
