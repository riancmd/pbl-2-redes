package main

import (
	"PlanoZ/models" // certifique-se q o caminho ta certo
	"fmt"
	"time"

	"github.com/fatih/color"
)

// logica da batalha (distribuida)

// essa eh a goroutine principal da batalha, ela q manda em tudo
// (esse server eh o "host" s1)
func (s *Server) iniciarBatalha(battleID string, b *models.Batalha, canalRespostaJ1 string) {
	color.Yellow("BATALHA (Host J1): Iniciando loop da batalha %s (%s vs %s)", battleID, b.Jogador1, b.Jogador2)

	// pega os dados do j2 pra gnt saber pra qm responder
	s.muPlayers.RLock()
	infoJ2, okJ2 := s.playerList[b.Jogador2]
	s.muPlayers.RUnlock()

	if !okJ2 {
		s.encerrarBatalha(battleID, "Ninguém", "J2 desconectou antes do início")
		return
	}

	// avisa o j1 q comecou (o j2 ja foi avisado pelo handleBattleInitiate)
	respInicioJ1 := models.RespostaInicioBatalha{
		Mensagem:  b.Jogador2,
		IdBatalha: battleID,
	}
	s.sendToClient(canalRespostaJ1, "Inicio_Batalha", respInicioJ1)

	time.Sleep(1 * time.Second) // da um segundinho pros clients respirarem

	turno := 0
	indice1, indice2 := 0, 0
	var carta1, carta2 *models.Tanque

	isSelfTest := b.ServidorJ1 == b.ServidorJ2 // checa se eh um teste local (j1 e j2 no msm server)

	for {
		// ve se alguem mandou a gnt parar (tipo o cleanup.go)
		select {
		case <-b.CanalEncerra:
			color.Red("BATALHA %s: Encerrada à força (via CanalEncerra)", battleID)
			return // encerrarbatalha ja foi chamado ou vai ser
		default:
		}

		//  pegar carta do j1 (local)
		if carta1 == nil {
			if indice1 >= 5 { // 5 cartas por partida
				s.encerrarBatalha(battleID, b.Jogador2, "J1 sem cartas")
				return
			}
			// pede a carta pro j1 (via redis)
			s.sendToClient(canalRespostaJ1, "Pedir_Carta", models.RespostaPedirCarta{Indice: indice1})

			// agora trava aqui e espera o j1 responder no canalj1
			// (quem bota a carta aqui eh o handlers_redis.go)
			novaCarta, ok := s.esperarCarta(b.CanalJ1, 20*time.Second) // 20s de timeout, se n responder ja era
			if !ok {
				s.encerrarBatalha(battleID, b.Jogador2, "Timeout J1")
				return
			}
			carta1 = novaCarta
			indice1++
		}

		//  pegar carta do j2 (remoto ou self-test)
		if carta2 == nil {
			if indice2 >= 5 {
				s.encerrarBatalha(battleID, b.Jogador1, "J2 sem cartas")
				return
			}

			if isSelfTest {
				// pede pro j2 localmente (via redis)
				s.sendToClient(infoJ2.ReplyChannel, "Pedir_Carta", models.RespostaPedirCarta{Indice: indice2})
			} else {
				// pede pro j2 la no outro server (via api)
				reqMove := models.BattleRequestMoveRequest{IdBatalha: battleID, Indice: indice2}
				if err := s.sendToHost(infoJ2.ServerHost, "/battle/request_move", reqMove); err != nil {
					s.encerrarBatalha(battleID, b.Jogador1, "Falha de rede ao pedir carta J2")
					return
				}
			}

			// agora espera o j2 responder no canalj2
			// (quem bota a carta aqui eh o handlers_api.go)
			novaCarta, ok := s.esperarCarta(b.CanalJ2, 20*time.Second) // 20s de timeout tbm
			if !ok {
				s.encerrarBatalha(battleID, b.Jogador1, "Timeout J2")
				return
			}
			carta2 = novaCarta
			indice2++
		}

		//  processar o turno
		var respTurno models.RespostaTurnoRealizado // cria a struct de resposta
		if turno%2 == 0 {                           // turno par, j1 ataca
			carta2.Vida -= carta1.Ataque
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (você) jogou no turno %d", b.Jogador1, turno)
		} else { // turno impar, j2 ataca
			carta1.Vida -= carta2.Ataque
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (oponente) jogou no turno %d", b.Jogador2, turno)
		}

		// bota as cartas na resposta
		respTurno.Cartas = []models.Tanque{*carta1, *carta2}

		// manda pro j1 (o sendtoclient bota o 'tipo' generico)
		s.sendToClient(canalRespostaJ1, "Turno_Realizado", respTurno)

		// ajusta a msg pro ponto de vista do j2
		if turno%2 == 0 {
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (oponente) jogou no turno %d", b.Jogador1, turno)
		} else {
			respTurno.Mensagem = fmt.Sprintf("Jogador %s (você) jogou no turno %d", b.Jogador2, turno)
		}

		if isSelfTest {
			// envia pro j2 (se for self-test)
			s.sendToClient(infoJ2.ReplyChannel, "Turno_Realizado", respTurno)
		} else {
			// envia pro j2 (remoto, via api)
			reqResult := models.BattleTurnResultRequest{IdBatalha: battleID, Resultado: respTurno}
			s.sendToHost(infoJ2.ServerHost, "/battle/turn_result", reqResult)
		}

		// ve se alguem morreu
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

// funcao helper q espera uma carta chegar no canal, ou da timeout
func (s *Server) esperarCarta(canal chan models.Tanque, tempo time.Duration) (*models.Tanque, bool) {
	timeout := time.After(tempo)
	select {
	case c, ok := <-canal:
		if !ok {
			return nil, false // canal fechou, deu ruim
		}
		return &c, true
	case <-timeout:
		return nil, false
	}
}

// funcao central pra limpar a batalha, fechar canais e avisar todo mundo
func (s *Server) encerrarBatalha(battleID, vencedor, motivo string) {
	// tira a batalha do map (impede novas jogadas)
	s.muBatalhas.Lock()
	batalha, ok := s.batalhas[battleID]
	if !ok { // ja foi encerrada, n faz nada
		s.muBatalhas.Unlock()
		return
	}
	delete(s.batalhas, battleID)
	s.muBatalhas.Unlock()

	// fecha os canais pra destravar as goroutines
	// manda um sinal nao-blocante pra goroutine da batalha parar (se ela ainda tiver la)
	select {
	case batalha.CanalEncerra <- true:
	default:
	}
	close(batalha.CanalEncerra)
	close(batalha.CanalJ1)
	close(batalha.CanalJ2)

	color.Yellow("BATALHA %s: Encerrada. Vencedor: %s. Motivo: %s", battleID, vencedor, motivo)

	// prepara a msg de fim de jogo
	respFim := models.RespostaFimBatalha{
		Mensagem: fmt.Sprintf("Batalha encerrada! Vencedor: %s (%s).", vencedor, motivo),
	}

	// avisa os jogadores
	s.muPlayers.RLock()
	infoJ1, okJ1 := s.playerList[batalha.Jogador1]
	infoJ2, okJ2 := s.playerList[batalha.Jogador2]
	s.muPlayers.RUnlock()

	if okJ1 {
		s.sendToClient(infoJ1.ReplyChannel, "Fim_Batalha", respFim)
	}

	if okJ2 {
		if batalha.ServidorJ1 == batalha.ServidorJ2 { // self-test, avisa local
			s.sendToClient(infoJ2.ReplyChannel, "Fim_Batalha", respFim)
		} else {
			// avisa o server 2 (remoto) pra ele avisar o j2 de la
			reqEnd := models.BattleEndRequest{IdBatalha: battleID, Resultado: respFim}
			s.sendToHost(infoJ2.ServerHost, "/battle/end", reqEnd) // se der erro aqui ja era, a batalha acabou msm
		}
	}
}
