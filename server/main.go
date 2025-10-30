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

	"PlanoZ/models" // Atualize este caminho se necessário

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// --- Constantes Globais ---
const (
	// Tópicos Globais do Redis
	TopicoConectar     = "conectar"
	TopicoComprarCarta = "comprar_carta"

	// Health Check
	HealthCheckInterval = 5 * time.Second
	RequestTimeout      = 2 * time.Second
)

// --- Pacote de Cartas (Original) ---
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

// --- Estruturas do Servidor ---

// PlayerInfo armazena onde cada jogador está conectado
type PlayerInfo struct {
	ServerID     string // ID do servidor (ex: "server1")
	ServerHost   string // Host:Port da API (ex: "server1:9090")
	ReplyChannel string // Canal de resposta do cliente (ex: "client_reply:UUID")
}

// peerBattleInfo armazena informações sobre uma batalha onde este servidor é o J2 (Peer)
type peerBattleInfo struct {
	PlayerID string // O ID do nosso jogador local (J2)
	HostAPI  string // O endereço da API do servidor Host (J1) que iniciou a batalha
}

// Server é a struct principal que detém todo o estado.
type Server struct {
	ID           string // ID deste servidor (ex: "server1")
	HostAPI      string // Meu endereço API (ex: "server1:9090")
	HostUDP      string // Meu endereço UDP (ex: "server1:8081") (host:porta)
	CanalPessoal string // Meu canal Redis (ex: "servidor_pessoal:server1")

	redisClient *redis.ClusterClient
	httpClient  *http.Client
	ginEngine   *gin.Engine
	ctx         context.Context

	// Estado Distribuído (Sincronizado pelo Líder)
	muPlayers     sync.RWMutex
	playerList    map[string]PlayerInfo // Chave: PlayerID
	muInventory   sync.RWMutex
	pacoteCounter int

	// Estado de Liderança
	muLeader      sync.RWMutex
	currentLeader string            // ID do líder (ex: "server1")
	serverList    map[string]string // Mapa de ID do servidor -> Host:Port API
	liveServers   map[string]bool   // Servidores atualmente ativos
	muLiveServers sync.RWMutex

	// Estado Local (Não sincronizado)
	muBatalhas sync.RWMutex
	batalhas   map[string]*models.Batalha // Chave: BattleID (para batalhas que EU hospedo)

	muBatalhasPeer sync.RWMutex
	batalhasPeer   map[string]peerBattleInfo // Chave: BattleID, Valor: peerBattleInfo
}

// --- Main: Inicialização ---

func main() {
	color.NoColor = false

	// 1. Carregar Configuração do Ambiente (Docker)
	serverID := getEnv("SERVER_ID", "server"+uuid.NewString()[:4])
	apiPort := getEnv("API_PORT", "9090")
	udpPort := getEnv("UDP_PORT", "8081")
	redisAddrs := getEnv("REDIS_ADDRS", "redis-node-1:6379,redis-node-2:6379,redis-node-3:6379")
	serverListStr := getEnv("SERVER_LIST", "server1:9090,server2:9091,server3:9092")

	// 2. Criar Cliente Redis
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: strings.Split(redisAddrs, ","),
	})
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao Redis: %v", err))
	}
	color.Green("Conectado ao Cluster Redis em %s", redisAddrs)

	// 3. Processar Lista de Servidores
	serverMap := make(map[string]string)
	for _, s := range strings.Split(serverListStr, ",") {
		parts := strings.Split(s, ":") // "server1:9090"
		if len(parts) == 2 {
			serverMap[parts[0]] = s
		}
	}

	// 4. Criar Instância do Servidor
	s := &Server{
		ID:            serverID,
		HostAPI:       fmt.Sprintf("%s:%s", serverID, apiPort), // "server1:9090"
		HostUDP:       fmt.Sprintf("%s:%s", serverID, udpPort), // "server1:8081"
		CanalPessoal:  fmt.Sprintf("servidor_pessoal:%s", serverID),
		redisClient:   rdb,
		httpClient:    &http.Client{Timeout: RequestTimeout},
		ctx:           ctx,
		playerList:    make(map[string]PlayerInfo),
		serverList:    serverMap,
		liveServers:   make(map[string]bool),
		batalhas:      make(map[string]*models.Batalha),
		batalhasPeer:  make(map[string]peerBattleInfo),
		pacoteCounter: 10, // Valor inicial
	}
	s.ginEngine = s.setupRouter() // Função do router.go

	// 5. Iniciar Serviços
	go s.RunRedisListeners()
	go s.RunAPI(apiPort)
	go s.RunUDP(udpPort)

	// 6. Esperar pelo "Enter" para iniciar a Eleição de Líder
	color.Yellow("Servidor %s pronto.", s.ID)
	color.Yellow("API rodando em :%s, UDP em :%s", apiPort, udpPort)
	color.Cyan("Pressione ENTER para iniciar a eleição de líder e os health checks...")
	bufio.NewReader(os.Stdin).ReadString('\n')

	// 7. Iniciar Eleição de Líder e Health Checks
	s.electNewLeader(nil)  // Eleição inicial (do leadership.go)
	go s.RunHealthChecks() // (do leadership.go)

	// Manter o programa rodando
	select {}
}

// --- Funções de Inicialização (Run) ---

// RunAPI inicia o servidor Gin (REST)
func (s *Server) RunAPI(port string) {
	color.Green("Iniciando servidor API Gin na porta :%s", port)
	// Ouve em "0.0.0.0:port"
	if err := s.ginEngine.Run(":" + port); err != nil {
		panic(fmt.Sprintf("Falha ao iniciar Gin: %v", err))
	}
}

// RunUDP inicia o servidor UDP para Ping/Heartbeat
func (s *Server) RunUDP(port string) {
	// Ouve em "0.0.0.0:port"
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
	s.lidarPing(udpConn) // (Função do utils.go)
}

// RunRedisListeners inicia as goroutines para ouvir os tópicos do Redis
func (s *Server) RunRedisListeners() {
	color.Green("Iniciando listeners do Redis...")
	go s.listenRedisGlobal(TopicoConectar)     // (do handlers_redis.go)
	go s.listenRedisGlobal(TopicoComprarCarta) // (do handlers_redis.go)
	go s.listenRedisPersonal()                 // (do handlers_redis.go)
}
