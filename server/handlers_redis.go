package main

import (
	"PlanoZ/models"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
)

// --- Listeners do Redis ---

// Ouve tópicos globais (conectar, comprar_carta)
func (s *Server) listenRedisGlobal(topico string) {
	color.Cyan("Ouvindo tópico global do Redis: %s", topico)
	for {
		resultado, err := s.redisClient.BLPop(s.ctx, 0*time.Second, topico).Result()
		if err != nil {
			color.Red("Erro ao ler do tópico %s: %v", topico, err)
			time.Sleep(1 * time.Second)
			continue
		}

		msg := resultado[1]

		if topico == TopicoConectar {
			var req models.ReqConectar
			if err := json.Unmarshal([]byte(msg), &req); err != nil {
				color.Red("Erro ao decodificar ReqConectar: %v", err)
				continue
			}
			go s.processConectar(req)
		} else if topico == TopicoComprarCarta {
			var req models.ReqComprarCarta
			if err := json.Unmarshal([]byte(msg), &req); err != nil {
				color.Red("Erro ao decodificar ReqComprarCarta: %v", err)
				continue
			}
			go s.processComprarCarta(req)
		}
	}
}

// Ouve o tópico pessoal deste servidor (parear, batalhar, jogada)
func (s *Server) listenRedisPersonal() {
	color.Cyan("Ouvindo tópico pessoal do Redis: %s", s.CanalPessoal)
	for {
		resultado, err := s.redisClient.BLPop(s.ctx, 0*time.Second, s.CanalPessoal).Result()
		if err != nil {
			color.Red("Erro ao ler do tópico pessoal %s: %v", s.CanalPessoal, err)
			time.Sleep(1 * time.Second)
			continue
		}

		msg := resultado[1]

		// Tenta decodificar como ReqPessoalServidor (parear, batalhar, msg)
		var reqPessoal models.ReqPessoalServidor
		errPessoal := json.Unmarshal([]byte(msg), &reqPessoal)

		if errPessoal == nil && reqPessoal.Tipo != "" {
			go s.processReqPessoal(reqPessoal)
			continue
		}

		// Se não for, tenta decodificar como ReqJogadaBatalha
		var reqJogada models.ReqJogadaBatalha
		errJogada := json.Unmarshal([]byte(msg), &reqJogada)

		if errJogada == nil && reqJogada.IdBatalha != "" {
			go s.processReqJogadaBatalha(reqJogada)
			continue
		}

		color.Red("Erro ao decodificar requisição pessoal. Pessoal: %v, Jogada: %v", errPessoal, errJogada)
	}
}

// --- Processadores de Requisições Redis ---

// Processa uma nova conexão de cliente
func (s *Server) processConectar(req models.ReqConectar) {
	color.Green("Processando conexão para %s", req.IdRemetente)

	leaderReq := models.LeaderConnectRequest{
		PlayerID:      req.IdRemetente,
		ServerID:      s.ID,
		CanalResposta: req.CanalResposta,
	}

	if s.isLeader() {
		// Se EU sou o líder, processo localmente
		s.muPlayers.Lock()
		oldInfo, exists := s.playerList[req.IdRemetente]
		playerInfo := PlayerInfo{
			ServerID:     s.ID,
			ServerHost:   s.HostAPI,
			ReplyChannel: req.CanalResposta,
		}
		s.playerList[req.IdRemetente] = playerInfo
		s.muPlayers.Unlock()
		color.Cyan("LÍDER: Jogador %s registrado (em mim mesmo)", req.IdRemetente)

		// Transmite a atualização para os outros servidores
		if exists && oldInfo.ServerID != s.ID {
			s.broadcastToServers("/players/update", models.UpdatePlayerListRequest{
				PlayerID: req.IdRemetente, ServerID: oldInfo.ServerID, Acao: "remove",
			})
		}
		s.broadcastToServers("/players/update", models.UpdatePlayerListRequest{
			PlayerID: req.IdRemetente, ServerID: s.ID, CanalResposta: req.CanalResposta, Acao: "add",
		})
	} else {
		// Se NÃO sou o líder, encaminho para ele
		if err := s.sendToLeader("/players/connect", leaderReq); err != nil {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Falha ao contatar o líder"})
			return
		}
	}

	// Responde ao cliente com sucesso
	resp := models.RespostaConexao{
		Mensagem:             "Conectado com sucesso",
		IdServidorConectado:  s.ID,
		CanalPessoalServidor: s.CanalPessoal,
		CanalUDPPing:         s.HostUDP, // Envia o "host:porta" UDP, ex: "server1:8081"
	}
	s.sendToClient(req.CanalResposta, "Conexao_Sucesso", resp)
}

// Processa uma compra de pacote
func (s *Server) processComprarCarta(req models.ReqComprarCarta) {
	color.Green("Processando compra de carta para %s", req.IdRemetente)

	leaderReq := models.LeaderBuyCardRequest{
		PlayerID: req.IdRemetente,
		ServerID: s.ID,
	}

	if s.isLeader() {
		// Se EU sou o líder, processo localmente
		s.muPlayers.RLock()
		playerInfo, ok := s.playerList[req.IdRemetente]
		s.muPlayers.RUnlock()
		if !ok {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Jogador não encontrado"})
			return
		}

		s.muInventory.Lock()
		if s.pacoteCounter <= 0 {
			s.muInventory.Unlock()
			s.sendToClient(playerInfo.ReplyChannel, "Erro", models.RespostaErro{Erro: "Não há mais pacotes disponíveis"})
			return
		}
		s.pacoteCounter--
		pacotesRestantes := s.pacoteCounter
		s.muInventory.Unlock()

		color.Cyan("LÍDER: Pacote vendido para %s. Restantes: %d", req.IdRemetente, pacotesRestantes)

		invUpdate := models.UpdateInventoryRequest{PacotesRestantes: pacotesRestantes}
		s.broadcastToServers("/inventory/update", invUpdate)

		cartas := s.sortearCartas(req.IdRemetente)
		respSorteio := models.RespostaSorteio{
			Mensagem: "Sorteio realizado com sucesso!",
			Cartas:   cartas,
		}
		s.sendToClient(playerInfo.ReplyChannel, "Sorteio", respSorteio)

	} else {
		// Se NÃO sou o líder, encaminho para ele
		if err := s.sendToLeader("/cards/buy", leaderReq); err != nil {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Falha ao contatar o líder"})
			return
		}
	}
}

// Processa requisições pessoais (Parear, Mensagem, Batalhar)
func (s *Server) processReqPessoal(req models.ReqPessoalServidor) {
	switch req.Tipo {
	case "Parear":
		color.Green("Processando pareamento para %s com %s", req.IdRemetente, req.IdDestinatario)
		s.muPlayers.RLock()
		infoDest, ok := s.playerList[req.IdDestinatario]
		s.muPlayers.RUnlock()

		if !ok {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Jogador destinatário não encontrado ou offline"})
			return
		}

		// Notifica o remetente
		respRemetente := models.RespostaPareamento{
			Mensagem:   fmt.Sprintf("Pareamento realizado com %s", req.IdDestinatario),
			IdParceiro: req.IdDestinatario,
		}
		s.sendToClient(req.CanalResposta, "Pareamento", respRemetente)

		// Notifica o destinatário
		respDestinatario := models.RespostaPareamento{
			Mensagem:   fmt.Sprintf("Pareamento realizado com %s", req.IdRemetente),
			IdParceiro: req.IdRemetente,
		}
		s.sendToClient(infoDest.ReplyChannel, "Pareamento", respDestinatario)

	case "Mensagem":
		color.Green("Processando msg de %s para %s", req.IdRemetente, req.IdDestinatario)
		s.muPlayers.RLock()
		infoDest, ok := s.playerList[req.IdDestinatario]
		s.muPlayers.RUnlock()

		if !ok {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Jogador destinatário não encontrado ou offline"})
			return
		}

		// Envia a mensagem para o destinatário
		respMsg := models.RespostaMensagem{
			Remetente: req.IdRemetente,
			Mensagem:  req.Mensagem,
		}
		s.sendToClient(infoDest.ReplyChannel, "Mensagem", respMsg)

	case "Batalhar":
		color.Green("Processando início de batalha entre %s e %s", req.IdRemetente, req.IdDestinatario)

		// Este servidor (S1) será o HOST da batalha.
		s.muPlayers.RLock()
		infoJ1, okJ1 := s.playerList[req.IdRemetente]
		infoJ2, okJ2 := s.playerList[req.IdDestinatario]
		s.muPlayers.RUnlock()

		if !okJ1 || !okJ2 {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Um dos jogadores não foi encontrado"})
			return
		}

		// 1. Criar a struct Batalha
		battleID := "battle:" + uuid.New().String()[:8]
		batalha := &models.Batalha{
			Jogador1:     req.IdRemetente,
			Jogador2:     req.IdDestinatario,
			ServidorJ1:   infoJ1.ServerHost,
			ServidorJ2:   infoJ2.ServerHost,
			CanalJ1:      make(chan models.Tanque, 1), // Canal com buffer 1
			CanalJ2:      make(chan models.Tanque, 1), // Canal com buffer 1
			CanalEncerra: make(chan bool, 1),
		}

		// 2. Armazenar a batalha localmente (como Host)
		s.muBatalhas.Lock()
		s.batalhas[battleID] = batalha
		s.muBatalhas.Unlock()

		// 3. Iniciar a goroutine da batalha (do battle.go)
		go s.iniciarBatalha(battleID, batalha, infoJ1.ReplyChannel)

		// 4. Notificar o Servidor J2 (Peer) para ele avisar o J2
		initReq := models.BattleInitiateRequest{
			IdBatalha:      battleID,
			IdJogadorLocal: req.IdDestinatario, // J2
			IdOponente:     req.IdRemetente,    // J1
			HostServidor:   s.HostAPI,          // Endereço de callback (EU, S1)
		}

		// Se for um self-test (J1 e J2 no mesmo server)
		if infoJ1.ServerID == infoJ2.ServerID {
			// Simula a chamada de rede localmente (chamando a lógica do handler)
			s.muBatalhasPeer.Lock()
			s.batalhasPeer[battleID] = peerBattleInfo{
				PlayerID: req.IdDestinatario,
				HostAPI:  s.HostAPI,
			}
			s.muBatalhasPeer.Unlock()

			respInicioJ2 := models.RespostaInicioBatalha{
				Mensagem:  req.IdRemetente,
				IdBatalha: battleID,
			}
			s.sendToClient(infoJ2.ReplyChannel, "Inicio_Batalha", respInicioJ2)
			color.Green("BATALHA (Self-Test): Batalha %s registrada para J2 %s", battleID, req.IdDestinatario)
		} else {
			// Chamada de rede normal para S2
			if err := s.sendToHost(infoJ2.ServerHost, "/battle/initiate", initReq); err != nil {
				s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Falha ao iniciar batalha com o servidor do oponente"})
				s.encerrarBatalha(battleID, "Ninguém", "Falha de Rede")
				return
			}
		}
	}
}

// Processa uma jogada de batalha (recebida do Redis)
func (s *Server) processReqJogadaBatalha(req models.ReqJogadaBatalha) {
	// Esta requisição pode ser de J1 (Host) ou J2 (Peer)

	// Tenta como Host (J1)
	s.muBatalhas.RLock()
	batalhaHost, okHost := s.batalhas[req.IdBatalha]
	s.muBatalhas.RUnlock()

	if okHost {
		// Verifica se é o J1 (Host) enviando
		if req.IdRemetente == batalhaHost.Jogador1 {
			select {
			case batalhaHost.CanalJ1 <- req.Carta:
				color.Green("BATALHA (Host J1): Recebida carta de J1 para batalha %s", req.IdBatalha)
			case <-time.After(2 * time.Second):
				color.Red("BATALHA (Host J1): Timeout ao enviar carta de J1 (canal cheio?) %s", req.IdBatalha)
			}
			return
		}
		// Verifica se é o J2 (Host-Self-Test) enviando
		if req.IdRemetente == batalhaHost.Jogador2 {
			select {
			case batalhaHost.CanalJ2 <- req.Carta:
				color.Green("BATALHA (Host J2-Self): Recebida carta de J2 para batalha %s", req.IdBatalha)
			case <-time.After(2 * time.Second):
				color.Red("BATALHA (Host J2-Self): Timeout ao enviar carta de J2 (canal cheio?) %s", req.IdBatalha)
			}
			return
		}
	}

	// Tenta como Peer (J2)
	s.muBatalhasPeer.RLock()
	peerInfo, okPeer := s.batalhasPeer[req.IdBatalha]
	s.muBatalhasPeer.RUnlock()

	if okPeer && req.IdRemetente == peerInfo.PlayerID {
		// É o J2 (Peer) enviando. Precisamos encaminhar para o Servidor Host (J1)
		submitReq := models.BattleSubmitMoveRequest{
			IdBatalha: req.IdBatalha,
			Carta:     req.Carta,
		}

		color.Green("BATALHA (Peer J2): Encaminhando jogada de %s para Host %s", req.IdRemetente, peerInfo.HostAPI)

		// Envia a jogada para o S1 (Host) via API
		if err := s.sendToHost(peerInfo.HostAPI, "/battle/submit_move", submitReq); err != nil {
			s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{
				Erro: fmt.Sprintf("Falha ao enviar jogada para o servidor host: %v", err),
			})
		}
		return
	}

	color.Red("BATALHA: Recebida jogada para batalha %s, mas batalha não encontrada como Host ou Peer.", req.IdBatalha)
	s.sendToClient(req.CanalResposta, "Erro", models.RespostaErro{Erro: "Batalha não encontrada ou já encerrada."})
}
