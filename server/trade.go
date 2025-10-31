package main

import (
	"PlanoZ/models"
	"fmt"
	"time"

	"github.com/fatih/color"
)

// --- Lógica da Troca (Distribuída) ---

// iniciarTroca é a goroutine principal que gerencia o estado de uma única troca.
// Ela é o "Host" (S1).
func (s *Server) iniciarTroca(tradeID string, t *models.Troca, canalRespostaJ1 string) {
	color.Yellow("TROCA (Host J1): Iniciando goroutine da troca %s (%s vs %s)", tradeID, t.Jogador1, t.Jogador2)

	// Pegar informações do J2 (necessário para enviar respostas)
	s.muPlayers.RLock()
	infoJ2, okJ2 := s.playerList[t.Jogador2]
	s.muPlayers.RUnlock()

	if !okJ2 {
		s.encerrarTroca(tradeID, "J2 desconectou antes do início")
		return
	}

	// Envia o início da troca para J1 (J2 é notificado pelo handleTradeInitiate)
	respInicioJ1 := models.RespostaInicioTroca{
		Mensagem: t.Jogador2,
		IdTroca:  tradeID,
	}
	s.sendToClient(canalRespostaJ1, "Inicio_Troca", respInicioJ1)

	isSelfTest := t.ServidorJ1 == t.ServidorJ2

	// Verifica se a goroutine foi instruída a parar (ex: por desconexão)
	select {
	case <-t.CanalEncerra:
		color.Red("TROCA %s: Encerrada à força (via CanalEncerra)", tradeID)
		return // limparTroca já foi (ou será) chamado
	default:
	}

	// --- Pedir Cartas (Simultaneamente) ---

	// 1. Pede a carta ao J1 (via Redis)
	s.sendToClient(canalRespostaJ1, "Pedir_Carta_Troca", models.RespostaPedirCartaTroca{IdTroca: tradeID})

	// 2. Pede a carta ao J2 (Remoto ou Local)
	if isSelfTest {
		// Pede a carta ao J2 localmente (via Redis)
		s.sendToClient(infoJ2.ReplyChannel, "Pedir_Carta_Troca", models.RespostaPedirCartaTroca{IdTroca: tradeID})
	} else {
		// Pede a carta ao J2 remotamente (via API para S2)
		reqCard := models.TradeRequestCardRequest{IdTroca: tradeID}
		if err := s.sendToHost(infoJ2.ServerHost, "/trade/request_card", reqCard); err != nil {
			s.encerrarTroca(tradeID, "Falha de rede ao pedir carta J2")
			return
		}
	}

	// --- Esperar Cartas ---

	// 3. Espera a carta de J1
	// (enviada por processReqCartaTroca)
	carta1, ok1 := s.esperarCartaTroca(t.CanalJ1, 30*time.Second) // Timeout de 30s
	if !ok1 {
		s.encerrarTroca(tradeID, "Timeout J1 (jogador demorou a escolher a carta)")
		return
	}

	// 4. Espera a carta de J2
	// (enviada por handleTradeSubmitCard ou processReqCartaTroca)
	carta2, ok2 := s.esperarCartaTroca(t.CanalJ2, 30*time.Second) // Timeout de 30s
	if !ok2 {
		s.encerrarTroca(tradeID, "Timeout J2 (jogador demorou a escolher a carta)")
		return
	}

	// --- Consumar a Troca ---
	color.Green("TROCA (Host J1): Cartas recebidas para %s. J1 enviou '%s', J2 enviou '%s'", tradeID, carta1.Modelo, carta2.Modelo)

	// 1. Notifica J1 (Local) sobre a carta que ele recebeu (Carta de J2)
	// O cliente J1, ao receber isso, deve atualizar seu inventário:
	// REMOVE carta1, ADICIONA carta2
	respJ1 := models.RespostaResultadoTroca{
		Mensagem:      fmt.Sprintf("Troca com %s concluída!", t.Jogador2),
		CartaRecebida: *carta2, // J1 recebe a carta de J2
	}
	s.sendToClient(canalRespostaJ1, "Resultado_Troca", respJ1)

	// 2. Notifica J2 (Remoto ou Local) sobre a carta que ele recebeu (Carta de J1)
	// O cliente J2, ao receber isso, deve atualizar seu inventário:
	// REMOVE carta2, ADICIONA carta1
	if isSelfTest {
		respJ2 := models.RespostaResultadoTroca{
			Mensagem:      fmt.Sprintf("Troca com %s concluída!", t.Jogador1),
			CartaRecebida: *carta1, // J2 recebe a carta de J1
		}
		s.sendToClient(infoJ2.ReplyChannel, "Resultado_Troca", respJ2)
	} else {
		// Envia para o Servidor S2, que chama o handleTradeResult
		reqResult := models.TradeResultRequest{
			IdTroca:       tradeID,
			CartaRecebida: *carta1, // S2/J2 recebe a carta de J1
		}
		// O handleTradeResult em S2 irá notificar o cliente J2 e limpará a troca em S2
		s.sendToHost(infoJ2.ServerHost, "/trade/result", reqResult)
	}

	// 3. Limpa a troca do Host (S1)
	s.limparTroca(tradeID)
}

// esperarCarta bloqueia até uma carta chegar no canal ou o timeout estourar.
// (Esta função é idêntica à de battle.go)
func (s *Server) esperarCartaTroca(canal chan models.Tanque, tempo time.Duration) (*models.Tanque, bool) {
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

// encerrarTroca é chamada APENAS EM CASO DE FALHA (ex: timeout, desconexão).
// Ela limpa a troca e notifica os jogadores sobre o cancelamento.
func (s *Server) encerrarTroca(tradeID string, motivo string) {
	// 1. Remove a troca do mapa (impede novas ações)
	t, ok := s.limparTroca(tradeID)
	if !ok {
		return // Troca já encerrada ou limpa
	}

	color.Red("TROCA %s: Encerrada por FALHA. Motivo: %s", tradeID, motivo)

	// 2. Prepara a notificação de Erro
	respErro := models.RespostaErro{
		Erro: fmt.Sprintf("Troca cancelada: %s", motivo),
	}

	// 3. Notifica os jogadores
	s.muPlayers.RLock()
	infoJ1, okJ1 := s.playerList[t.Jogador1]
	infoJ2, okJ2 := s.playerList[t.Jogador2]
	s.muPlayers.RUnlock()

	// 4. Notifica J1 (Local)
	if okJ1 {
		s.sendToClient(infoJ1.ReplyChannel, "Erro", respErro)
	}

	// 5. Notifica J2 (Remoto ou Local)
	if okJ2 {
		if t.ServidorJ1 == t.ServidorJ2 { // Self-test
			s.sendToClient(infoJ2.ReplyChannel, "Erro", respErro)
		} else {
			// Notifica o Servidor J2 para ele notificar o J2
			// Usamos o handleTradeResult, mas enviamos uma carta "vazia"
			// O handler S2 (handleTradeResult) deve limpar sua referência 'tradesPeer'
			// e notificar seu cliente J2 sobre o erro.
			reqEnd := models.TradeResultRequest{
				IdTroca:       tradeID,
				CartaRecebida: models.Tanque{}, // Envia uma carta vazia para indicar falha
			}
			s.sendToHost(infoJ2.ServerHost, "/trade/result", reqEnd) // Ignora erro aqui
		}
	}
}

// limparTroca remove a troca do mapa e fecha os canais.
// Esta é a função de limpeza interna do Host (S1).
func (s *Server) limparTroca(tradeID string) (*models.Troca, bool) {
	// 1. Remove a troca do mapa
	s.muTrades.Lock()
	troca, ok := s.trades[tradeID]
	if !ok { // Troca já encerrada
		s.muTrades.Unlock()
		return nil, false
	}
	delete(s.trades, tradeID)
	s.muTrades.Unlock()

	// 2. Fecha os canais (sinaliza para a goroutine parar, se estiver presa)
	select {
	case troca.CanalEncerra <- true:
	default:
	}
	close(troca.CanalEncerra)
	close(troca.CanalJ1)
	close(troca.CanalJ2)

	color.Yellow("TROCA (Host J1): Troca %s limpa do mapa e canais fechados.", tradeID)
	return troca, true
}
