package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"PlanoZ/models" // certifique-se q o caminho ta certo

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// constantes globais
const (
	// topicos globais do redis
	TopicoConectar     = "conectar"
	TopicoComprarCarta = "comprar_carta"

	// configs do health check
	HealthCheckInterval = 5 * time.Second
	RequestTimeout      = 2 * time.Second
)

// pacote de cartas inicial
var pacote_1 = []models.Tanque{
	{"M22 (Light)", "server", 50, 10}, {"M22 (Light)", "server", 50, 10}, {"M22 (Light)", "server", 50, 10},
	{"FIAT6614 (Light)", "server", 55, 12}, {"FIAT6614 (Light)", "server", 55, 12}, {"FIAT6614 (Light)", "server", 55, 12},
	{"BMP (Light)", "server", 60, 15}, {"BMP (Light)", "server", 60, 15}, {"BMP (Light)", "server", 60, 15},
	{"Fox (Light)", "server", 52, 11}, {"Fox (Light)", "server", 52, 11}, {"Fox (Light)", "server", 52, 11},
	{"AMX13 (Light)", "server", 58, 14}, {"AMX13 (Light)", "server", 58, 14}, {"AMX13 (Light)", "server", 58, 14},
	{"Sherman (Medium)", "server", 100, 28}, {"Sherman (Medium)", "server", 100, 28},
	{"T-34 (Medium)", "server", 110, 27}, {"T-34 (Medium)", "server", 110, 27},
	{"Panther (Medium)", "server", 120, 25}, {"Panther (Medium)", "server", 120, 25},
	{"M47 (Medium)", "server", 115, 30}, {"M47 (Medium)", "server", 115, 30},
	{"Tiger II (Heavy)", "server", 200, 53}, {"IS-6 (Heavy)", "server", 220, 55},
	{"M26 Pershing (Heavy)", "server", 210, 52}, {"T-10M (Heavy)", "server", 230, 58},
	{"KV-2 (Heavy)", "server", 250, 50}, {"Maus (Heavy)", "server", 280, 57},
	{"M26E5 (Heavy)", "server", 240, 54},
}

// structs do servidor

// info de onde o player ta conectado
type PlayerInfo struct {
	ServerID     string // ex: "server1"
	ServerHost   string // ex: "server1:9090"
	ReplyChannel string // ex: "client_reply:UUID"
}

// info da batalha qnd a gnt eh o j2 (peer)
type peerBattleInfo struct {
	PlayerID string // o id do nosso jogador local (j2)
	HostAPI  string // o endereço da api do server host (j1)
}

// info da troca qnd a gnt eh o j2 (peer)
type peerTradeInfo struct {
	PlayerID string // id do nosso jogador (j2)
	HostAPI  string // api do server host (j1)
}

// A struct principal do server. tem tudo aqui dentro
type Server struct {
	ID           string // ex: "server1"
	HostAPI      string // ex: "server1:9090" (api)
	HostUDP      string // ex: "server1:8081" (udp)
	CanalPessoal string // ex: "servidor_pessoal:server1" (redis)

	redisClient *redis.ClusterClient
	httpClient  *http.Client
	ginEngine   *gin.Engine
	ctx         context.Context

	// estado global (sincronizado pelo lider)
	muPlayers     sync.RWMutex
	playerList    map[string]PlayerInfo // map[playerID] -> PlayerInfo
	muInventory   sync.RWMutex
	pacoteCounter int

	// estado de lideranca
	muLeader      sync.RWMutex
	currentLeader string            // ex: "server1"
	serverList    map[string]string // map[serverID] -> host:porta
	liveServers   map[string]bool   // map[serverID] -> ta vivo?
	muLiveServers sync.RWMutex

	// estado local (coisas q so esse server precisa saber)
	muBatalhas     sync.RWMutex
	batalhas       map[string]*models.Batalha // batalhas q *eu* hospedo (eu sou o s1)
	muBatalhasPeer sync.RWMutex
	batalhasPeer   map[string]peerBattleInfo // batalhas q *outro* server hospeda (eu sou o s2)

	muTrades     sync.RWMutex
	trades       map[string]*models.Troca // trocas q *eu* hospedo
	muTradesPeer sync.RWMutex
	tradesPeer   map[string]peerTradeInfo // trocas q *outro* server hospeda
}

// main: inicializacao
func main() {
	color.NoColor = false

	// carrega as config das variaveis de ambiente (docker-compose)
	serverID := getEnv("SERVER_ID", "server"+uuid.NewString()[:4])
	apiPort := getEnv("API_PORT", "9090")
	udpPort := getEnv("UDP_PORT", "8081")
	redisAddrs := getEnv("REDIS_ADDRS", "redis-node-1:6379,redis-node-2:6379,redis-node-3:6379")
	serverListStr := getEnv("SERVER_LIST", "server1:9090,server2:9091,server3:9092")

	// conecta no cluster redis
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: strings.Split(redisAddrs, ","),
	})
	ctx := context.Background()
	// testa a conexao com o redis
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao Redis: %v", err))
	}
	color.Green("Conectado ao Cluster Redis em %s", redisAddrs)

	// *** pausa critica ***
	// da um tempo pro 'redis-cluster-init' terminar de criar o cluster
	// se a gnt tentar ler (blpop) antes disso, da o erro 'clusterdown'
	initialWait := 15 * time.Second
	color.Yellow("Aguardando %v para estabilização do cluster Redis...", initialWait)
	time.Sleep(initialWait)

	// le a lista de todos os servers (do env)
	serverMap := make(map[string]string)
	for _, s := range strings.Split(serverListStr, ",") {
		parts := strings.Split(s, ":") // "server1:9090"
		if len(parts) == 2 {
			serverMap[parts[0]] = s
		}
	}

	// cria a struct principal do server
	s := &Server{
		ID:            serverID,
		HostAPI:       fmt.Sprintf("%s:%s", serverID, apiPort), // "server1:9090"
		HostUDP:       fmt.Sprintf("%s:%s", serverID, udpPort), // "server1:8081" (importante pro cliente)
		CanalPessoal:  fmt.Sprintf("servidor_pessoal:%s", serverID),
		redisClient:   rdb,
		httpClient:    &http.Client{Timeout: RequestTimeout},
		ctx:           ctx,
		playerList:    make(map[string]PlayerInfo),
		serverList:    serverMap,
		liveServers:   make(map[string]bool),
		batalhas:      make(map[string]*models.Batalha),
		batalhasPeer:  make(map[string]peerBattleInfo),
		trades:        make(map[string]*models.Troca),
		tradesPeer:    make(map[string]peerTradeInfo),
		pacoteCounter: 10, // estoque inicial
	}
	s.ginEngine = s.setupRouter() // prepara as rotas da api (do router.go)

	// inicia as goroutines principais
	go s.RunRedisListeners() // goroutine pra ouvir o redis
	go s.RunAPI(apiPort)     // goroutine pra servir a api http
	go s.RunUDP(udpPort)     // goroutine pro udp (ping/heartbeat)

	// espera o admin dar enter no terminal
	color.Yellow("Servidor %s pronto.", s.ID)
	color.Yellow("API rodando em :%s, UDP em :%s", apiPort, udpPort)
	color.Cyan("Pressione ENTER para iniciar a eleição de líder e os health checks...")
	bufio.NewReader(os.Stdin).ReadString('\n')

	// agora sim, comeca a eleicao
	go s.RunHealthChecks() // (do leadership.go)
	s.electNewLeader(nil)  // (do leadership.go)

	// trava a main thread aqui pra sempre
	select {}
}

// funcoes de inicializacao (run)

// inicia o servidor http (gin)
func (s *Server) RunAPI(port string) {
	color.Green("Iniciando servidor API Gin na porta :%s", port)
	// ouve em "0.0.0.0:port"
	if err := s.ginEngine.Run(":" + port); err != nil {
		panic(fmt.Sprintf("Falha ao iniciar Gin: %v", err))
	}
}

// inicia o servidor udp (ping)
func (s *Server) RunUDP(port string) {
	// ouve em "0.0.0.0:port"
	udpAddr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		color.Red("Erro ao resolver porta UDP %s: %v", port, err)
		return
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		color.Red("Erro na criação da porta UDP %s: %v", port, err)
		return
	}
	defer udpConn.Close()
	color.Green("Iniciando servidor UDP Ping em :%s", port)
	s.lidarPing(udpConn) // (do utils.go)
}

// inicia as 3 goroutines q ouvem o redis
func (s *Server) RunRedisListeners() {
	color.Green("Iniciando listeners do Redis...")
	go s.listenRedisGlobal(TopicoConectar)     // (do handlers_redis.go)
	go s.listenRedisGlobal(TopicoComprarCarta) // (do handlers_redis.go)
	go s.listenRedisPersonal()                 // (do handlers_redis.go)
}
