package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync/atomic" // Para o controle de estado do heartbeat
	"time"

	"PlanoZ/models"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Estados da máquina de estados
const (
	EstadoLivre = iota
	EstadoPareado
	EstadoEsperandoResposta
	EstadoBatalhando
	EstadoReconectando // Estado para quando o heartbeat falha
)

// Variáveis de sessão do cliente
var (
	idPessoal            string // UUID gerado pelo cliente
	idParceiro           string // ID do oponente pareado
	idBatalha            string // ID da batalha atual
	minhasCartas         []models.Tanque
	estadoAtual          int
	meuCanalResposta     string // Canal de Resposta Pessoal (ex: client_reply:UUID)
	canalPessoalServidor string // Canal de Requisição do Servidor (ex: servidor_pessoal:SERVER_ID_123)
	canalUdpServidor     string // Endereço UDP do servidor (ex: server1:8081)

	// Contexto e Cliente Redis
	ctx         = context.Background()
	redisClient *redis.ClusterClient

	// Variáveis de Heartbeat
	serverVivo    atomic.Bool        // Controla se o servidor conectado está vivo
	monitorCancel context.CancelFunc // Função para cancelar o monitor UDP anterior
)

// unmarshalData é um helper para decodificar o campo 'Data' da RespostaGenericaCliente
func unmarshalData(data interface{}, v interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(dataBytes, v)
}

// enviarRequisicaoRedis serializa e envia (LPUSH) uma requisição para um tópico do Redis
func enviarRequisicaoRedis(topico string, data interface{}) {
	reqBytes, err := json.Marshal(data)
	if err != nil {
		color.Red("Erro ao serializar requisição: %v", err)
		return
	}

	err = redisClient.LPush(ctx, topico, reqBytes).Err()
	if err != nil {
		color.Red("Erro ao enviar requisição para Redis (Tópico: %s): %v", topico, err)
	}
}

// iniciarMonitoramentoHeartbeat roda em segundo plano para verificar se o servidor está vivo.
func iniciarMonitoramentoHeartbeat(ctxMonitor context.Context, endereco string) {
	ticker := time.NewTicker(5 * time.Second) // Pinga a cada 5 segundos
	defer ticker.Stop()

	falhasConsecutivas := 0
	const maxFalhas = 3

	color.Green("[Heartbeat]: Monitoramento UDP iniciado para %s", endereco)

	for {
		select {
		case <-ctxMonitor.Done():
			// O contexto foi cancelado (provavelmente conectamos a um novo server)
			color.Yellow("[Heartbeat]: Monitoramento UDP encerrado para %s", endereco)
			return
		case <-ticker.C:
			// No docker-compose, o host 'server1' pode não ser roteável
			// A forma correta é o servidor enviar seu endereço IP roteável ou
			// o cliente usar o nome do host do Docker.
			// Para simplificar, vamos assumir que o 'endereco' é apenas a porta
			// e que estamos conectando ao 'host.docker.internal' ou localhost.
			// No Docker Compose, o nome do serviço (ex: 'server1') é o host.

			// A porta UDP já vem como "8081". Precisamos do host.
			// No docker compose, o host é o nome do serviço.
			// Mas o cliente não sabe o nome do serviço...

			// *** ASSUNÇÃO CRÍTICA ***
			// Vamos assumir que 'canalUdpServidor' (o 'endereco' aqui)
			// é o endereço COMPLETO roteável, ex: "server1:8081".
			// O servidor em 'processConectar' deve enviar isso.
			// (Vou modificar o server/handlers_redis.go para enviar o HOST:PORT)

			_, err := medirLatenciaUnica(endereco)
			if err != nil {
				falhasConsecutivas++
				color.Red("[Heartbeat]: Falha no ping (%d/%d): %v", falhasConsecutivas, maxFalhas, err)
				if falhasConsecutivas >= maxFalhas {
					// 3 falhas seguidas = servidor morto
					color.Red("[Heartbeat]: Servidor %s considerado MORTO.", endereco)
					serverVivo.Store(false) // Sinaliza para a thread principal
					return                  // Encerra esta goroutine
				}
			} else {
				// Ping bem-sucedido
				if falhasConsecutivas > 0 {
					color.Green("[Heartbeat]: Servidor %s recuperado.", endereco)
				}
				falhasConsecutivas = 0
				serverVivo.Store(true) // Garante que está marcado como vivo
			}
		}
	}
}

// ouvirRespostasRedis é a goroutine principal que escuta (BLPOP) no canal de resposta pessoal.
func ouvirRespostasRedis() {
	deckBatalha := make([]models.Tanque, 0, 5) // Deck de batalha local

	for {
		// BLPop bloqueia até que uma mensagem chegue no 'meuCanalResposta'
		resultado, err := redisClient.BLPop(ctx, 0*time.Second, meuCanalResposta).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			// Se o Redis cair, o cliente não tem como se recuperar
			color.Red("Erro crítico ao ler do Redis: %v. Encerrando.", err)
			os.Exit(1)
		}

		// resultado[0] é a chave (o nome do tópico), resultado[1] é a mensagem
		var resposta models.RespostaGenericaCliente
		err = json.Unmarshal([]byte(resultado[1]), &resposta)
		if err != nil {
			color.Red("Erro ao deserializar resposta genérica: %v", err)
			continue
		}

		// Processar a resposta com base no Tipo
		switch resposta.Tipo {
		case "Erro":
			var resp models.RespostaErro
			if unmarshalData(resposta.Data, &resp) == nil {
				color.Red("Erro do Servidor: %s", resp.Erro)
			}
			// Retorna ao estado anterior com base no contexto
			if idParceiro == "none" {
				estadoAtual = EstadoLivre
			} else {
				estadoAtual = EstadoPareado
			}

		case "Desconexão":
			// Esta é uma desconexão "limpa" (ex: oponente saiu da batalha)
			color.Yellow("Parece que seu jogador pareado desconectou :(")
			estadoAtual = EstadoLivre
			idParceiro = "none"
			idBatalha = "none"

		case "Conexao_Sucesso":
			var resp models.RespostaConexao
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaConexao")
				continue
			}
			color.Green("Conectado com sucesso! Servidor: %s", resp.IdServidorConectado)
			canalPessoalServidor = resp.CanalPessoalServidor
			canalUdpServidor = resp.CanalUDPPing // Ex: "server1:8081"
			estadoAtual = EstadoLivre

			// Ativa o monitoramento de heartbeat para o novo servidor
			serverVivo.Store(true)

			// Cancela qualquer monitor anterior que esteja rodando
			if monitorCancel != nil {
				monitorCancel()
			}

			// Cria um novo contexto e inicia um novo monitor
			var ctxMonitor context.Context
			ctxMonitor, monitorCancel = context.WithCancel(context.Background())
			go iniciarMonitoramentoHeartbeat(ctxMonitor, resp.CanalUDPPing)

		case "Pareamento":
			var resp models.RespostaPareamento
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaPareamento")
				continue
			}
			color.Green("Pareamento realizado com %s", resp.IdParceiro)
			idParceiro = resp.IdParceiro
			estadoAtual = EstadoPareado

		case "Mensagem":
			var resp models.RespostaMensagem
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaMensagem")
				continue
			}
			color.Cyan("Mensagem de [%s]: %s", resp.Remetente, resp.Mensagem)

		case "Sorteio":
			var resp models.RespostaSorteio
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaSorteio")
				continue
			}
			minhasCartas = append(minhasCartas, resp.Cartas...)
			color.Green("%s\n", resp.Mensagem)
			imprimirTanques(resp.Cartas)

		case "Inicio_Batalha":
			var resp models.RespostaInicioBatalha
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaInicioBatalha")
				continue
			}
			color.Yellow("Batalha iniciada! Oponente: %s. ID da Batalha: %s", resp.Mensagem, resp.IdBatalha)
			idBatalha = resp.IdBatalha // Armazena o ID da batalha
			deckBatalha = nil
			if len(minhasCartas) > 0 {
				deckBatalha = append(deckBatalha, sortearDeck()...)
			} else {
				// Inicializa deck de batalha com cartas inoperantes
				for i := 0; i < 5; i++ {
					deckBatalha = append(deckBatalha, models.Tanque{Modelo: "Treinamento", Id_jogador: idPessoal, Vida: 1 + i, Ataque: 1})
				}
			}
			color.Cyan("Seu deck de batalha é:")
			imprimirTanques(deckBatalha)
			estadoAtual = EstadoBatalhando

		case "Fim_Batalha":
			var resp models.RespostaFimBatalha
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaFimBatalha")
				continue
			}
			color.Yellow("Batalha finalizada!")
			color.Cyan(resp.Mensagem)

			// Se a batalha terminou, mas o servidor está morto,
			// não vá para EstadoPareado, vá para Reconectar.
			if serverVivo.Load() {
				estadoAtual = EstadoPareado
			} else {
				estadoAtual = EstadoReconectando
			}
			idBatalha = "none" // Reseta o ID da batalha

		case "Pedir_Carta":
			var resp models.RespostaPedirCarta
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaPedirCarta")
				continue
			}
			indice := resp.Indice
			var carta models.Tanque

			// Verificar se indice é válido
			if indice < 0 || indice >= len(deckBatalha) {
				color.Red("ERRO: Índice %d fora do range do deck (0-%d). Enviando carta padrão.", indice, len(deckBatalha)-1)
				carta = models.Tanque{Modelo: "Padrão", Vida: 10, Ataque: 1, Id_jogador: idPessoal}
			} else {
				carta = deckBatalha[indice]
				color.Cyan("Enviando carta: %s (Vida: %d, Ataque: %d)", carta.Modelo, carta.Vida, carta.Ataque)
			}

			reqJogada := models.ReqJogadaBatalha{
				IdRemetente:   idPessoal,
				CanalResposta: meuCanalResposta,
				IdBatalha:     idBatalha,
				Carta:         carta,
			}
			enviarRequisicaoRedis(canalPessoalServidor, reqJogada)

		case "Turno_Realizado":
			var resp models.RespostaTurnoRealizado
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaTurnoRealizado")
				continue
			}
			color.Yellow("Turno Realizado!")
			color.Yellow(resp.Mensagem)
			imprimirTanques(resp.Cartas)

		default:
			color.Red("Resposta recebida com tipo desconhecido: %s", resposta.Tipo)
		}
	}
}

func main() {
	color.NoColor = false

	// 1. Gerar ID do Cliente e Canal de Resposta
	idPessoal = uuid.New().String()
	meuCanalResposta = "client_reply:" + idPessoal
	color.Yellow("Meu ID Pessoal: %s", idPessoal)
	color.Yellow("Meu Canal de Resposta: %s", meuCanalResposta)

	// 2. Conectar ao Cluster Redis
	// (Estes endereços viriam do Docker Compose)
	redisAddrs := []string{"redis-node-1:6379", "redis-node-2:6379", "redis-node-3:6379"}
	redisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: redisAddrs,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		color.Red("Falha ao conectar ao cluster Redis: %v", err)
		panic(err)
	}
	color.Green("Conectado ao cluster Redis em: %v", redisAddrs)

	// 3. Iniciar Goroutine para ouvir respostas
	go ouvirRespostasRedis()

	// 4. Enviar requisição de conexão inicial
	reqConnect := models.ReqConectar{
		IdRemetente:   idPessoal,
		CanalResposta: meuCanalResposta,
	}
	enviarRequisicaoRedis("conectar", reqConnect)

	// Estado inicial
	estadoAtual = EstadoEsperandoResposta
	idParceiro = "none"
	idBatalha = "none"
	serverVivo.Store(true) // Otimista, o heartbeat corrigirá se estiver errado

	// 5. Loop infinito e centralizado que lê do terminal
	reader := bufio.NewReader(os.Stdin)
	for {
		// *** VERIFICAÇÃO DE HEARTBEAT (Ação Local) ***
		// No início de cada loop, verifica se o monitor UDP marcou o servidor como morto
		if !serverVivo.Load() && estadoAtual != EstadoEsperandoResposta && estadoAtual != EstadoReconectando {
			color.Red("\n!!! CONEXÃO COM O SERVIDOR PERDIDA !!!")

			if estadoAtual == EstadoBatalhando || estadoAtual == EstadoPareado {
				color.Yellow("Você foi deslogado. Batalha ou pareamento interrompido.")
			} else {
				color.Yellow("Você foi deslogado.")
			}

			// *** LIMPA AS VARIÁVEIS LOCAIS (Conforme solicitado) ***
			estadoAtual = EstadoReconectando
			idParceiro = "none"
			idBatalha = "none"
			canalPessoalServidor = ""
			canalUdpServidor = ""

			// Para o monitoramento antigo
			if monitorCancel != nil {
				monitorCancel()
				monitorCancel = nil
			}
		}
		// *** FIM DA VERIFICAÇÃO ***

		// Ver qual estado do jogador
		switch estadoAtual {
		case EstadoLivre:
			fmt.Println("Comando Parear <id> / Abrir / Ping / Sair: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "Sair" {
				os.Exit(0)
			}

			if strings.HasPrefix(line, "Parear ") {
				idDestinatario := strings.TrimPrefix(line, "Parear ")
				req := models.ReqPessoalServidor{
					Tipo:           "Parear",
					IdRemetente:    idPessoal,
					CanalResposta:  meuCanalResposta,
					IdDestinatario: idDestinatario,
				}
				enviarRequisicaoRedis(canalPessoalServidor, req)
				estadoAtual = EstadoEsperandoResposta

			} else if strings.HasPrefix(line, "Abrir") {
				req := models.ReqComprarCarta{
					IdRemetente:   idPessoal,
					CanalResposta: meuCanalResposta,
				}
				enviarRequisicaoRedis("comprar_carta", req)

			} else if strings.HasPrefix(line, "Ping") {
				if canalUdpServidor == "" {
					color.Red("Endereço UDP do servidor ainda não recebido.")
				} else {
					handleManualPing(reader)
				}
			} else {
				color.Red("Comando inválido")
			}

		case EstadoPareado:
			fmt.Println("Comando Abrir / Mensagem / Batalhar / Ping / Sair: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "Sair" {
				os.Exit(0)
			}

			if strings.HasPrefix(line, "Abrir") {
				req := models.ReqComprarCarta{
					IdRemetente:   idPessoal,
					CanalResposta: meuCanalResposta,
				}
				enviarRequisicaoRedis("comprar_carta", req)

			} else if strings.HasPrefix(line, "Batalhar") {
				if len(minhasCartas) < 5 {
					color.Red("Você não tem cartas suficientes para montar um deck")
				} else {
					req := models.ReqPessoalServidor{
						Tipo:           "Batalhar",
						IdRemetente:    idPessoal,
						CanalResposta:  meuCanalResposta,
						IdDestinatario: idParceiro,
					}
					enviarRequisicaoRedis(canalPessoalServidor, req)
					estadoAtual = EstadoEsperandoResposta
				}
			} else if strings.HasPrefix(line, "Mensagem ") {
				mensagem := strings.TrimPrefix(line, "Mensagem ")
				req := models.ReqPessoalServidor{
					Tipo:           "Mensagem",
					IdRemetente:    idPessoal,
					CanalResposta:  meuCanalResposta,
					IdDestinatario: idParceiro,
					Mensagem:       mensagem,
				}
				enviarRequisicaoRedis(canalPessoalServidor, req)

			} else if strings.HasPrefix(line, "Ping") {
				if canalUdpServidor == "" {
					color.Red("Endereço UDP do servidor ainda não recebido.")
				} else {
					handleManualPing(reader)
				}
			} else {
				color.Red("Comando inválido")
			}

		case EstadoEsperandoResposta:
			color.Yellow("Esperando resposta do server...")
			time.Sleep(1 * time.Second)

		case EstadoBatalhando:
			color.Yellow("Batalha ocorrendo!! (Aguardando instruções do servidor...)")
			time.Sleep(5 * time.Second)

		case EstadoReconectando:
			color.Yellow("Tentando reconectar a um novo servidor...")
			// Envia uma nova requisição de conexão.
			// O `BLPOP` do Redis garante que um servidor *vivo* pegue isso.
			reqConnect := models.ReqConectar{
				IdRemetente:   idPessoal,
				CanalResposta: meuCanalResposta,
			}
			enviarRequisicaoRedis("conectar", reqConnect)

			// Otimista: assume que vai funcionar. O heartbeat irá corrigir se
			// o novo servidor também estiver morto.
			serverVivo.Store(true)

			// Muda para o estado de espera pela resposta "Conexao_Sucesso"
			estadoAtual = EstadoEsperandoResposta
			time.Sleep(3 * time.Second) // Evita spamming de reconexão

		default:
			color.Red("Estado indefinido")
		}
	}
}

// --- Funções Utilitárias (Jogo) ---

// Função para sortear 5 cartas da coleção de cartas do jogador
func sortearDeck() []models.Tanque {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	n := len(minhasCartas)
	indices := r.Perm(n)[:5]

	deck := make([]models.Tanque, 0, 5)
	for _, i := range indices {
		deck = append(deck, minhasCartas[i])
	}
	return deck
}

// Função para imprimir a lista de tanques/cartas
func imprimirTanques(lista []models.Tanque) {
	for i, t := range lista {
		fmt.Printf("Tanque %d:\n", i+1)
		fmt.Printf("  Modelo: %s\n", t.Modelo)
		color.Yellow("  Jogador: %s", t.Id_jogador)
		color.Green("  Vida: %d", t.Vida)
		color.Red("  Ataque: %d", t.Ataque)
	}
}

// --- Funções de Ping UDP (Simplificadas) ---

// handleManualPing é chamado pelo usuário para ver a latência
func handleManualPing(reader *bufio.Reader) {
	color.Cyan("Medindo Ping (UDP) para %s...", canalUdpServidor)
	latencia, err := medirLatenciaUnica(canalUdpServidor)
	if err != nil {
		color.Red("Falha na medição: %v", err)
	} else {
		color.Yellow("Latência: %s", latencia.String())
	}
	fmt.Println("Pressione Enter para continuar...")
	reader.ReadString('\n') // Espera o usuário pressionar Enter
}

// medirLatenciaUnica envia um "ping" UDP e espera um "pong" UDP
func medirLatenciaUnica(endereco string) (time.Duration, error) {
	// 'endereco' deve ser "host:porta", ex: "server1:8081"

	// 1. Resolver endereço
	servAddr, err := net.ResolveUDPAddr("udp", endereco)
	if err != nil {
		return 0, fmt.Errorf("falha ao resolver endereço UDP '%s': %w", endereco, err)
	}

	// 2. "Discar" (apenas define o destino padrão)
	conn, err := net.DialUDP("udp", nil, servAddr)
	if err != nil {
		return 0, fmt.Errorf("falha ao discar UDP: %w", err)
	}
	defer conn.Close()

	// 3. Definir timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	startTime := time.Now()

	// 4. Enviar "ping" (string simples)
	_, err = conn.Write([]byte("ping"))
	if err != nil {
		return 0, fmt.Errorf("erro ao enviar ping UDP: %w", err)
	}

	// 5. Aguardar "pong" (string simples)
	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return 0, fmt.Errorf("timeout ou erro ao ler pong: %w", err)
	}

	resposta := string(buffer[:n])
	if resposta != "pong" {
		return 0, fmt.Errorf("resposta inesperada do servidor: %s", resposta)
	}

	return time.Since(startTime), nil
}
