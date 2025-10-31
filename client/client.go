package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic" // pra controlar o estado do heartbeat (thread-safe)
	"time"

	"PlanoZ/models" // nossas structs

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// os "modos" do cliente, como se fossem telas diferentes
const (
	EstadoLivre = iota
	EstadoPareado
	EstadoEsperandoResposta
	EstadoBatalhando
	EstadoTrocando
	EstadoReconectando // estado novo pra qnd o server cair
)

// variaveis globais pra guardar o estado do jogo
var (
	idPessoal            string // nosso ID unico, gerado qnd a gente abre o jogo
	idParceiro           string // id do maluco q a gente ta pareado
	idBatalha            string // id da sala de batalha q a gente ta
	idTroca              string // id da sala de troca
	minhasCartas         []models.Tanque
	indiceCartaOfertada  int    // pra saber qual carta a gente mandou na troca
	estadoAtual          int    // onde a gente ta agora (EstadoLivre, EstadoBatalhando, etc)
	meuCanalResposta     string // canal pessoal no redis. o server manda respostas pra ca
	canalPessoalServidor string // canal do server q a gente ta conectado, pra mandar reqs
	canalUdpServidor     string // o ip:porta do udp do server, pra pingar

	// coisas do redis
	ctx         = context.Background()
	redisClient *redis.ClusterClient

	// o "kill switch". a goroutine do heartbeat bota isso pra 'false' se o server cair
	serverVivo    atomic.Bool
	monitorCancel context.CancelFunc // pra parar a goroutine de heartbeat antiga qnd a gente reconecta
)

// funçaozinha helper. pega o 'Data' generico (interface{}) e bota na struct certa
func unmarshalData(data interface{}, v interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(dataBytes, v)
}

// serializa qualquer struct e envia pra uma lista/fila do redis (LPUSH)
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

// essa é a goroutine do heartbeat, fica pingando o server via udp
func iniciarMonitoramentoHeartbeat(ctxMonitor context.Context, endereco string) {
	ticker := time.NewTicker(5 * time.Second) // a cada 5 segundos...
	defer ticker.Stop()

	falhasConsecutivas := 0
	const maxFalhas = 3

	color.Green("[Heartbeat]: Monitoramento UDP iniciado para %s", endereco)

	for {
		select {
		case <-ctxMonitor.Done():
			// se a main thread mandou parar (pq reconectamos)
			color.Yellow("[Heartbeat]: Monitoramento UDP encerrado para %s", endereco)
			return
		case <-ticker.C:
			// hora de pingar

			_, err := medirLatenciaUnica(endereco) // ...tenta pingar
			if err != nil {
				falhasConsecutivas++ // falhou
				color.Red("[Heartbeat]: Falha no ping (%d/%d): %v", falhasConsecutivas, maxFalhas, err)
				if falhasConsecutivas >= maxFalhas {
					// falhou 3x seguidas, ja era
					color.Red("[Heartbeat]: Servidor %s considerado MORTO.", endereco)
					serverVivo.Store(false) // avisa a main loop q o server morreu
					return                  // e para essa goroutine
				}
			} else {
				// ping deu bom
				if falhasConsecutivas > 0 {
					color.Green("[Heartbeat]: Servidor %s recuperado.", endereco)
				}
				falhasConsecutivas = 0 // zera o contador
				serverVivo.Store(true) // garante q ta vivo
			}
		}
	}
}

// A GOROUTINE MAIS IMPORTANTE. fica ouvindo o nosso canal pessoal de respostas
func ouvirRespostasRedis() {
	deckBatalha := make([]models.Tanque, 0, 5) // deck de batalha local

	for {
		// aqui o codigo TRAVA, esperando o proximo BLPop no nosso canal
		resultado, err := redisClient.BLPop(ctx, 0*time.Second, meuCanalResposta).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			// se o redis cair, ja era
			color.Red("Erro crítico ao ler do Redis: %v. Encerrando.", err)
			os.Exit(1)
		}

		// qnd chega, tenta ler a msg generica
		var resposta models.RespostaGenericaCliente
		err = json.Unmarshal([]byte(resultado[1]), &resposta)
		if err != nil {
			color.Red("Erro ao deserializar resposta genérica: %v", err)
			continue
		}

		// agora vamos ver o q o server realmente quer dizer
		switch resposta.Tipo {
		case "Erro":
			// o server mandou um "deu ruim"
			var resp models.RespostaErro
			if unmarshalData(resposta.Data, &resp) == nil {
				color.Red("Erro do Servidor: %s", resp.Erro)
			}
			// volta pro menu
			if idParceiro == "none" {
				estadoAtual = EstadoLivre
			} else {
				estadoAtual = EstadoPareado
			}
			//idTroca = "none"

		case "Desconexão":
			// o oponente desconectou (de forma limpa)
			color.Yellow("Parece que seu jogador pareado desconectou :(")
			estadoAtual = EstadoLivre
			idParceiro = "none"
			idBatalha = "none"
			idTroca = "none"

		case "Conexao_Sucesso":
			// conseguimos conectar! o server mandou os dados dele
			var resp models.RespostaConexao
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaConexao")
				continue
			}
			color.Green("Conectado com sucesso! Servidor: %s", resp.IdServidorConectado)
			canalPessoalServidor = resp.CanalPessoalServidor // guarda o canal de reqs do server
			canalUdpServidor = resp.CanalUDPPing             // guarda o udp pra pingar
			estadoAtual = EstadoLivre                        // libera o menu principal

			// ativa o monitoramento de heartbeat
			serverVivo.Store(true)

			// se tinha um monitor antigo rodando (de uma conexao anterior), mata ele
			if monitorCancel != nil {
				monitorCancel()
			}

			// ...e comeca um monitor NOVO pra esse server
			var ctxMonitor context.Context
			ctxMonitor, monitorCancel = context.WithCancel(context.Background())
			go iniciarMonitoramentoHeartbeat(ctxMonitor, resp.CanalUDPPing)

		case "Pareamento":
			// achamos um oponente
			var resp models.RespostaPareamento
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaPareamento")
				continue
			}
			color.Green("Pareamento realizado com %s", resp.IdParceiro)
			idParceiro = resp.IdParceiro
			estadoAtual = EstadoPareado

		case "Mensagem":
			// chat
			var resp models.RespostaMensagem
			if unmarshalData(resposta.Data, &resp) == nil {
				color.Red("Falha ao ler RespostaMensagem")
				continue
			}
			color.Cyan("Mensagem de [%s]: %s", resp.Remetente, resp.Mensagem)

		case "Sorteio":
			// compramos um pacote, adiciona as cartas no inventario
			var resp models.RespostaSorteio
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaSorteio")
				continue
			}
			minhasCartas = append(minhasCartas, resp.Cartas...)
			color.Green("%s\n", resp.Mensagem)
			imprimirTanques(resp.Cartas)

		case "Inicio_Batalha":
			// comecou a batalha
			var resp models.RespostaInicioBatalha
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaInicioBatalha")
				continue
			}
			color.Yellow("Batalha iniciada! Oponente: %s. ID da Batalha: %s", resp.Mensagem, resp.IdBatalha)
			idBatalha = resp.IdBatalha // guarda o id da sala
			deckBatalha = nil
			if len(minhasCartas) > 0 {
				deckBatalha = append(deckBatalha, sortearDeck()...) // sorteia 5 cartas do nosso inventario
			} else {
				// se n tem carta, bota umas de treino
				for i := 0; i < 5; i++ {
					deckBatalha = append(deckBatalha, models.Tanque{Modelo: "Treinamento", Id_jogador: idPessoal, Vida: 1 + i, Ataque: 1})
				}
			}
			color.Cyan("Seu deck de batalha é:")
			imprimirTanques(deckBatalha)
			estadoAtual = EstadoBatalhando // muda a "tela" pra de batalha

		case "Inicio_Troca":
			// comecou a troca
			var resp models.RespostaInicioTroca
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaInicioTroca")
				continue
			}
			color.Magenta("Troca iniciada! Oponente: %s. ID da Troca: %s", resp.Mensagem, resp.IdTroca)
			idTroca = resp.IdTroca       // guarda o id da sala de troca
			indiceCartaOfertada = -1     // reseta o indice
			estadoAtual = EstadoTrocando // muda pra "tela" de troca

		case "Fim_Batalha":
			// acabou a luta
			var resp models.RespostaFimBatalha
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaFimBatalha")
				continue
			}
			color.Yellow("Batalha finalizada!")
			color.Cyan(resp.Mensagem)

			// checa se o server ainda ta vivo antes de voltar pro menu
			if serverVivo.Load() {
				estadoAtual = EstadoPareado
			} else {
				estadoAtual = EstadoReconectando
			}
			idBatalha = "none" // limpa o id da batalha

		case "Resultado_Troca":
			// a troca foi concluida (ou falhou)
			var resp models.RespostaResultadoTroca
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaResultadoTroca")
				continue
			}

			// se n veio carta, eh pq falhou ou foi cancelada
			if resp.CartaRecebida.Modelo == "" {
				color.Red("A troca falhou ou foi cancelada pelo outro jogador.")
			} else if indiceCartaOfertada < 0 || indiceCartaOfertada >= len(minhasCartas) {
				// deu algum erro interno de indice (nao devia acontecer)
				color.Red("ERRO INTERNO: Não foi possível encontrar a carta ofertada (índice: %d)", indiceCartaOfertada)
				minhasCartas = append(minhasCartas, resp.CartaRecebida) // Pelo menos adiciona a nova
			} else {
				// deu certo! hora de trocar as cartas no inventario
				cartaRemovida := minhasCartas[indiceCartaOfertada]

				// tira a carta antiga
				minhasCartas = append(minhasCartas[:indiceCartaOfertada], minhasCartas[indiceCartaOfertada+1:]...)

				// bota a carta nova
				minhasCartas = append(minhasCartas, resp.CartaRecebida)

				color.Green("Troca Concluída!")
				color.Red("  - REMOVIDO: %s", cartaRemovida.Modelo)
				color.Green("  + ADICIONADO: %s", resp.CartaRecebida.Modelo)
			}

			if serverVivo.Load() {
				estadoAtual = EstadoPareado
			} else {
				estadoAtual = EstadoReconectando
			}
			idTroca = "none" // limpa o id da troca

		case "Pedir_Carta":
			// O SERVER TA PEDINDO NOSSA JOGADA (BATALHA)
			var resp models.RespostaPedirCarta
			if unmarshalData(resposta.Data, &resp) != nil {
				color.Red("Falha ao ler RespostaPedirCarta")
				continue
			}
			indice := resp.Indice // o servidor so manda o *indice* q ele quer do nosso deck de batalha
			var carta models.Tanque

			// ve se o indice é valido
			if indice < 0 || indice >= len(deckBatalha) {
				color.Red("ERRO: Índice %d fora do range do deck (0-%d). Enviando carta padrão.", indice, len(deckBatalha)-1)
				carta = models.Tanque{Modelo: "Padrão", Vida: 10, Ataque: 1, Id_jogador: idPessoal}
			} else {
				carta = deckBatalha[indice] // acha a carta
				color.Cyan("Enviando carta: %s (Vida: %d, Ataque: %d)", carta.Modelo, carta.Vida, carta.Ataque)
			}

			reqJogada := models.ReqJogadaBatalha{ // prepara o pacote com a carta
				IdRemetente:   idPessoal,
				CanalResposta: meuCanalResposta,
				IdBatalha:     idBatalha,
				Carta:         carta,
			}
			enviarRequisicaoRedis(canalPessoalServidor, reqJogada) // e manda pro server

		case "Pedir_Carta_Troca":
			// o server ta pronto pra receber nossa oferta
			// a gente so avisa o usuario, quem le o input é o loop main
			color.Magenta("O servidor está pronto para receber sua oferta de troca.")
			color.Magenta("Use 'list' para ver suas cartas ou 'ofertar <indice>' para enviar.")

		case "Turno_Realizado":
			// o oponente jogou, so mostra o resultado
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

// o loop principal q le oq o usuario digita
func main() {
	color.NoColor = false

	// cria nosso id e nosso canal de "email"
	idPessoal = uuid.New().String()
	meuCanalResposta = "client_reply:" + idPessoal
	color.Yellow("Meu ID Pessoal: %s", idPessoal)
	color.Yellow("Meu Canal de Resposta: %s", meuCanalResposta)

	// conecta no redis
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

	//  IMPORTANTE: inicia a goroutine de escuta (o email)
	go ouvirRespostasRedis()

	// manda a primeira msg "OI, QUERO CONECTAR"
	reqConnect := models.ReqConectar{
		IdRemetente:   idPessoal,
		CanalResposta: meuCanalResposta,
	}
	enviarRequisicaoRedis("conectar", reqConnect)

	// estado inicial, esperando o "Conexao_Sucesso"
	estadoAtual = EstadoEsperandoResposta
	idParceiro = "none"
	idBatalha = "none"
	idTroca = "none"
	serverVivo.Store(true) // otimismo! acha q o server ta vivo. o heartbeat corrige se n tiver

	// o loop da ui (o menu)
	reader := bufio.NewReader(os.Stdin)
	for {
		//  O CHECK DO HEARTBEAT
		// se a goroutine do heartbeat (UDP) falou q o server morreu...
		if !serverVivo.Load() && estadoAtual != EstadoEsperandoResposta && estadoAtual != EstadoReconectando {
			color.Red("\n!!! CONEXÃO COM O SERVIDOR PERDIDA !!!")

			if estadoAtual == EstadoBatalhando || estadoAtual == EstadoPareado {
				color.Yellow("Você foi deslogado. Batalha ou pareamento interrompido.")
			} else {
				color.Yellow("Você foi deslogado.")
			}

			// LIMPANDO TUDO
			estadoAtual = EstadoReconectando // ...muda o estado pra reconectar
			idParceiro = "none"
			idBatalha = "none"
			idTroca = "none"
			canalPessoalServidor = ""
			canalUdpServidor = ""

			// ...mata a goroutine de ping antiga
			if monitorCancel != nil {
				monitorCancel()
				monitorCancel = nil
			}
		}
		//  FIM DA VERIFICAÇÃO

		// o menu principal (maquina de estados)
		switch estadoAtual {
		case EstadoLivre:
			// menu principal qnd n ta em batalha/pareado
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
			// menu qnd ta pareado com alguem
			fmt.Println("Comando Abrir / Mensagem / Batalhar / Trocar / Ping / Sair: ")
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
			} else if strings.HasPrefix(line, "Trocar") {
				// inicia o fluxo de troca
				if len(minhasCartas) == 0 {
					color.Red("Você não tem nenhuma carta para trocar.")
				} else {
					req := models.ReqPessoalServidor{
						Tipo:           "Trocar",
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
			// tela de "carregando..."
			color.Yellow("Esperando resposta do server...")
			time.Sleep(1 * time.Second)

		case EstadoBatalhando:
			// em batalha, o usuario n digita, so espera o server pedir a carta
			color.Yellow("Batalha ocorrendo!! (Aguardando instruções do servidor...)")
			time.Sleep(5 * time.Second)

		case EstadoTrocando:
			// TELA INTERATIVA DA TROCA
			color.Magenta("Troca em andamento com %s.", idParceiro)
			color.Magenta("Digite 'list' para ver suas cartas, 'ofertar <indice>' para enviar, ou 'cancelar'.")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "list" {
				// mostra o inventario
				if len(minhasCartas) == 0 {
					color.Yellow("Você não tem cartas.")
				} else {
					imprimirTanques(minhasCartas)
				}
			} else if line == "cancelar" {
				// cancela local, o server vai dar timeout sozinho
				color.Red("Troca cancelada localmente.")
				estadoAtual = EstadoPareado
				idTroca = "none"
				// O servidor (iniciarTroca) vai dar timeout e chamar encerrarTroca,
				// que enviará um Resultado_Troca vazio ou Erro, que será ignorado
				// pois já saímos do estado.
			} else if strings.HasPrefix(line, "ofertar ") {
				// o usuario quer ofertar uma carta
				indiceStr := strings.TrimPrefix(line, "ofertar ")
				// o usuário digita "ofertar 1" (base 1), mas o slice é base 0.
				indice, err := strconv.Atoi(indiceStr)
				if err != nil || indice <= 0 || indice > len(minhasCartas) {
					color.Red("Índice inválido. Digite um número entre 1 e %d.", len(minhasCartas))
				} else {
					indiceCartaOfertada = indice - 1 // converte o indice (usuario digita 1, mas o slice eh 0)
					carta := minhasCartas[indiceCartaOfertada]

					color.Cyan("Ofertando carta: %s (Vida: %d, Ataque: %d)", carta.Modelo, carta.Vida, carta.Ataque)

					// prepara o pacote pra enviar
					reqTroca := models.ReqCartaTroca{
						IdRemetente:   idPessoal,
						CanalResposta: meuCanalResposta,
						IdTroca:       idTroca,
						Carta:         carta,
					}
					enviarRequisicaoRedis(canalPessoalServidor, reqTroca) // envia pro server

					// agora eh so esperar o resultado da troca
					estadoAtual = EstadoEsperandoResposta
				}
			} else {
				color.Red("Comando inválido. Use 'list', 'ofertar <indice>' ou 'cancelar'.")
			}

		case EstadoReconectando:
			// o server caiu
			color.Yellow("Tentando reconectar a um novo servidor...")
			// manda um "OI, QUERO CONECTAR" de novo. algum server vivo vai pegar
			reqConnect := models.ReqConectar{
				IdRemetente:   idPessoal,
				CanalResposta: meuCanalResposta,
			}
			enviarRequisicaoRedis("conectar", reqConnect)

			// otimismo
			serverVivo.Store(true)

			// e espera a resposta
			estadoAtual = EstadoEsperandoResposta
			time.Sleep(3 * time.Second) // evita spam de conexao

		default:
			color.Red("Estado indefinido")
		}
	}
}

// --- Funções Utilitárias (Jogo) ---

// sorteia 5 cartas do nosso inventario pra levar pra batalha
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

// so imprime as cartas de um jeito bonito
func imprimirTanques(lista []models.Tanque) {
	for i, t := range lista {
		fmt.Printf("Tanque %d:\n", i+1) // i+1 pra ficar base 1 pro usuario (1, 2, 3...)
		fmt.Printf("  Modelo: %s\n", t.Modelo)
		color.Yellow("  Jogador: %s", t.Id_jogador)
		color.Green("  Vida: %d", t.Vida)
		color.Red("  Ataque: %d", t.Ataque)
	}
}

// --- Funções de Ping UDP (Simplificadas) ---

// o comando "Ping" do menu
func handleManualPing(reader *bufio.Reader) {
	color.Cyan("Medindo Ping (UDP) para %s...", canalUdpServidor)
	latencia, err := medirLatenciaUnica(canalUdpServidor)
	if err != nil {
		color.Red("Falha na medição: %v", err)
	} else {
		color.Yellow("Latência: %s", latencia.String())
	}
	fmt.Println("Pressione Enter para continuar...")
	reader.ReadString('\n') // espera o usuário pressionar enter pra voltar pro menu
}

// a funcao q faz o ping udp de vdd. envia "ping", espera "pong"
func medirLatenciaUnica(endereco string) (time.Duration, error) {
	// 'endereco' deve ser "host:porta", ex: "server1:8081"

	// acha o server
	servAddr, err := net.ResolveUDPAddr("udp", endereco)
	if err != nil {
		return 0, fmt.Errorf("falha ao resolver endereço UDP '%s': %w", endereco, err)
	}

	// "disca" (basicamente so prepara o pacote)
	conn, err := net.DialUDP("udp", nil, servAddr)
	if err != nil {
		return 0, fmt.Errorf("falha ao discar UDP: %w", err)
	}
	defer conn.Close()

	// bota um timeout, ninguem merece esperar pra sempre
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	startTime := time.Now()

	// manda "ping" (string simples)
	_, err = conn.Write([]byte("ping"))
	if err != nil {
		return 0, fmt.Errorf("erro ao enviar ping UDP: %w", err)
	}

	// espera "pong" (string simples)
	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return 0, fmt.Errorf("timeout ou erro ao ler pong: %w", err)
	}

	resposta := string(buffer[:n])
	if resposta != "pong" {
		// se o server n responder "pong", deu ruim
		return 0, fmt.Errorf("resposta inesperada do servidor: %s", resposta)
	}

	return time.Since(startTime), nil
}
