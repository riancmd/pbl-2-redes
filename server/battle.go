package main

import (
	"PlanoZ/models"
	"fmt"
	"time"

	"github.com/fatih/color"
)

// --- Lógica da Batalha (Distribuída) ---

// iniciarBatalha é a goroutine principal que gerencia o estado de uma única batalha.
// Ela é o "Host" (S1).
func (s *Server) iniciarBatalha(battleID string, b *models.Batalha, canalRespostaJ1 string) {
	color.Yellow("BATALHA (Host J1): Iniciando loop da batalha %s (%s vs %s)", battleID, b.Jogador1, b.Jogador2)

	// Pegar informações do J2 (necessário para enviar respostas)
	s.muPlayers.RLock()
	infoJ2, okJ2 := s.playerList[b.Jogador2]
	s.muPlayers.RUnlock()

	if !okJ2 {
		s.encerrarBatalha(battleID, "Ninguém", "J2 desconectou antes do início")
		return
	}

	// Envia o início da batalha para J1 (J2 é notificado pelo handleBattleInitiate)
	respInicioJ1 := models.RespostaInicioBatalha{
		Mensagem:  b.Jogador2,
		IdBatalha: battleID,
	}
	s.sendToClient(canalRespostaJ1, "Inicio_Batalha", respInicioJ1)

	time.Sleep(1 * time.Second) // Dar tempo para os clientes carregarem

	turno := 0
	indice1, indice2 := 0, 0
	var carta1, carta2 *models.Tanque

	isSelfTest := b.ServidorJ1 == b.ServidorJ2

	for {
		// Verifica se a goroutine foi instruída a parar (ex: por desconexão)
		select {
		case <-b.CanalEncerra:
			color.Red("BATALHA %s: Encerrada à força (via CanalEncerra)", battleID)
			return // encerrarBatalha já foi (ou será) chamado
		default:
		}

		// --- Obter Carta J1 (Local) ---
		if carta1 == nil {
			if indice1 >= 5 {
				s.encerrarBatalha(battleID, b.Jogador2, "J1 sem cartas")
				return
			}
			// 1. Pede a carta ao J1 (via Redis)
			s.sendToClient(canalRespostaJ1, "Pedir_Carta", models.RespostaPedirCarta{Indice: indice1})

			// 2. Espera a carta chegar no CanalJ1 (enviada por processReqJogadaBatalha)
			novaCarta, ok := s.esperarCarta(b.CanalJ1, 20*time.Second) // Timeout de 20s
			if !ok {
				s.encerrarBatalha(battleID, b.Jogador2, "Timeout J1")
				return
			}
			carta1 = novaCarta
			indice1++
		}

		// --- Obter Carta J2 (Remoto ou Local) ---
		if carta2 == nil {
			if indice2 >= 5 {
				s.encerrarBatalha(battleID, b.Jogador1, "J2 sem cartas")
				return
			}

			if isSelfTest {
				// 1. Pede a carta ao J2 localmente (via Redis)
				s.sendToClient(infoJ2.ReplyChannel, "Pedir_Carta", models.RespostaPedirCarta{Indice: indice2})
			} else {
				// 1. Pede a carta ao J2 remotamente (via API para S2)
				reqMove := models.BattleRequestMoveRequest{IdBatalha: battleID, Indice: indice2}
				if err := s.sendToHost(infoJ2.ServerHost, "/battle/request_move", reqMove); err != nil {
					s.encerrarBatalha(battleID, b.Jogador1, "Falha de rede ao pedir carta J2")
					return
				}
			}

			// 2. Espera a carta chegar no CanalJ2 (enviada por handleBattleSubmitMove)
			novaCarta, ok := s.esperarCarta(b.CanalJ2, 20*time.Second) // Timeout de 20s
			if !ok {
				s.encerrarBatalha(battleID, b.Jogador1, "Timeout J2")
				return
			}
			carta2 = novaCarta
			indice2++
		}

		// --- Processar Turno ---
		var respTurno models.RespostaTurnoRealizado // Cria a struct de resposta específica
		if turno%2 == 0 {                           // J1 ataca
			carta2.Vida -= carta1.Ataque
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (você) jogou no turno %d", b.Jogador1, turno)
		} else { // J2 ataca
			carta1.Vida -= carta2.Ataque
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (oponente) jogou no turno %d", b.Jogador2, turno)
		}

		// Preenche os outros campos da struct específica
		respTurno.Cartas = []models.Tanque{*carta1, *carta2}

		// Envia a struct específica para J1, envolvida pela genérica
		s.sendToClient(canalRespostaJ1, "Turno_Realizado", respTurno)

		// Ajusta a mensagem na *mesma* struct para a perspectiva do J2
		if turno%2 == 0 {
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (oponente) jogou no turno %d", b.Jogador1, turno)
		} else {
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (você) jogou no turno %d", b.Jogador2, turno)
		}

		if isSelfTest {
			// Envia a struct ajustada para J2 local, envolvida pela genérica
			s.sendToClient(infoJ2.ReplyChannel, "Turno_Realizado", respTurno)
		} else {
			// Envia a struct ajustada para J2 remoto, envolvida pela de comunicação REST
			reqResult := models.BattleTurnResultRequest{IdBatalha: battleID, Resultado: respTurno}
			s.sendToHost(infoJ2.ServerHost, "/battle/turn_result", reqResult)
		}

		// Verificar mortes
		if carta1.Vida <= 0 {
			carta1 = nil
		}
		if carta2.Vida <= 0 {
			carta2 = nil
		}

		turno++
		time.Sleep(1 * time.Second)
	}
}

// esperarCarta bloqueia até uma carta chegar no canal ou o timeout estourar.
// (Esta função e encerrarBatalha continuam no mesmo arquivo battle.go)
func (s *Server) esperarCarta(canal chan models.Tanque, tempo time.Duration) (*models.Tanque, bool) {
	timeout := time.After(tempo)
	select {
	case c, ok := <-canal:
		if !ok {
			return nil, false // Canal foi fechado
		}
		return &c, true
	case <-timeout:
		return nil, false
	}
}

// encerrarBatalha limpa a batalha do mapa, fecha os canais e notifica os jogadores.
// (Esta função continua no mesmo arquivo battle.go)
func (s *Server) encerrarBatalha(battleID, vencedor, motivo string) {
	// 1. Remove a batalha do mapa (impede novas jogadas)
	s.muBatalhas.Lock()
	batalha, ok := s.batalhas[battleID]
	if !ok { // Batalha já encerrada
		s.muBatalhas.Unlock()
		return
	}
	delete(s.batalhas, battleID)
	s.muBatalhas.Unlock()

	// 2. Fecha os canais (sinaliza para a goroutine parar)
	// (Usar non-blocking send no CanalEncerra para evitar deadlock se já foi fechado)
	select {
	case batalha.CanalEncerra <- true:
	default:
	}
	close(batalha.CanalEncerra)
	close(batalha.CanalJ1)
	close(batalha.CanalJ2)

	color.Yellow("BATALHA %s: Encerrada. Vencedor: %s. Motivo: %s", battleID, vencedor, motivo)

	// 3. Prepara a notificação de Fim de Batalha
	respFim := models.RespostaFimBatalha{
		Mensagem: fmt.Sprintf("Batalha encerrada! Vencedor: %s (%s).", vencedor, motivo),
	}

	// 4. Notifica os jogadores
	s.muPlayers.RLock()
	infoJ1, okJ1 := s.playerList[batalha.Jogador1]
	infoJ2, okJ2 := s.playerList[batalha.Jogador2]
	s.muPlayers.RUnlock()

	if okJ1 {
		s.sendToClient(infoJ1.ReplyChannel, "Fim_Batalha", respFim)
	}

	if okJ2 {
		if batalha.ServidorJ1 == batalha.ServidorJ2 { // Self-test
			s.sendToClient(infoJ2.ReplyChannel, "Fim_Batalha", respFim)
		} else {
			// Notifica o Servidor J2 para ele notificar o J2
			reqEnd := models.BattleEndRequest{IdBatalha: battleID, Resultado: respFim}
			s.sendToHost(infoJ2.ServerHost, "/battle/end", reqEnd) // Ignora erro aqui
		}
	}
}
