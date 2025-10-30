package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/fatih/color"
)

// --- Eleição de Líder e Health Check ---

// RunHealthChecks periodicamente verifica a saúde de outros servidores
func (s *Server) RunHealthChecks() {
	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.muLeader.RLock()
		leader := s.currentLeader
		s.muLeader.RUnlock()

		// Mapa temporário dos servidores que estão vivos *agora*
		liveNow := make(map[string]bool)
		
		var wg sync.WaitGroup
		for id, host := range s.serverList {
			wg.Add(1)
			go func(id, host string) {
				defer wg.Done()
				if s.checkServerHealth(host) {
					s.muLiveServers.Lock() // Protege a escrita no mapa
					liveNow[id] = true
					s.muLiveServers.Unlock()
				}
			}(id, host)
		}
		wg.Wait()

		// Lista de servidores que acabaram de morrer
		deadServers := []string{}
		
		s.muLiveServers.Lock()
		// Itera sobre o mapa *antigo* (s.liveServers)
		for id := range s.liveServers {
			if !liveNow[id] {
				// Se não está no mapa *novo* (liveNow), morreu.
				deadServers = append(deadServers, id)
			}
		}
		s.liveServers = liveNow // Atualiza o estado global para o mapa novo
		s.muLiveServers.Unlock()

		if len(deadServers) > 0 {
			color.Magenta("Servidores detectados como MORTOS: %v", deadServers)
			// Chama a função de limpeza (do cleanup.go)
			go s.limparRecursosServidoresMortos(deadServers)
		}

		// Lógica de reeleição (agora usa o mapa 'liveNow' atualizado)
		leaderIsAlive := liveNow[leader]
		if !leaderIsAlive && leader != "" {
			color.Red("Líder %s está OFFLINE. Iniciando nova eleição.", leader)
			s.electNewLeader(liveNow) // Passa o mapa atualizado
		}
	}
}

// checkServerHealth envia um GET /health para outro servidor
func (s *Server) checkServerHealth(host string) bool {
	if host == s.HostAPI { // Saúde própria
		return true
	}

	resp, err := s.httpClient.Get(fmt.Sprintf("http://%s/health", host))
	if err != nil {
		// (Comentado para não poluir o log)
		// color.Red("Falha no health check para %s: %v", host, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// electNewLeader elege o novo líder com base no ID alfabético (menor porta)
func (s *Server) electNewLeader(liveNow map[string]bool) {
	liveIDs := []string{}
	
	// Se for a eleição inicial, o mapa liveNow é nil
	if liveNow == nil {
		s.muLiveServers.RLock()
		for id := range s.liveServers {
			liveIDs = append(liveIDs, id)
		}
		s.muLiveServers.RUnlock()
	} else {
		for id := range liveNow {
			liveIDs = append(liveIDs, id)
		}
	}

	// Adiciona a si mesmo (caso o health check de si próprio ainda não tenha rodado)
	// (Verifica se já não está na lista primeiro)
	isMeInList := false
	for _, id := range liveIDs {
		if id == s.ID {
			isMeInList = true
			break
		}
	}
	
	if !isMeInList {
		if s.checkServerHealth(s.HostAPI) {
			liveIDs = append(liveIDs, s.ID)
		}
	}

	if len(liveIDs) == 0 {
		color.Red("Nenhum servidor vivo detectado, incluindo eu mesmo. Assumindo liderança por padrão.")
		liveIDs = append(liveIDs, s.ID)
	}

	sort.Strings(liveIDs) // "server1" < "server2"
	newLeaderID := liveIDs[0]

	s.muLeader.Lock()
	if s.currentLeader != newLeaderID {
		s.currentLeader = newLeaderID
		color.Green("NOVO LÍDER ELEITO: %s", s.currentLeader)
	}
	s.muLeader.Unlock()
}

// isLeader verifica se este servidor é o líder
func (s *Server) isLeader() bool {
	s.muLeader.RLock()
	defer s.muLeader.RUnlock()
	return s.currentLeader == s.ID
}