package models

// #############################################################################
// # ESTRUTURAS DO JOGO
// #############################################################################

// Tanque (Carta do jogo)
type Tanque struct {
	Modelo     string `json:"modelo"`
	Id_jogador string `json:"id_jogador"`
	Vida       int    `json:"vida"`
	Ataque     int    `json:"ataque"`
}

// Batalha (Armazenada no map 's.batalhas' do servidor Host)
// Contém os canais de comunicação para a goroutine da batalha.
type Batalha struct {
	Jogador1     string      `json:"jogador1"`
	Jogador2     string      `json:"jogador2"`
	ServidorJ1   string      `json:"servidor_j1"` // Endereço API do Host (ex: "server1:9090")
	ServidorJ2   string      `json:"servidor_j2"` // Endereço API do Peer (ex: "server2:9091")
	CanalJ1      chan Tanque `json:"-"`           // Canal para receber a carta do Jogador 1 (local)
	CanalJ2      chan Tanque `json:"-"`           // Canal para receber a carta do Jogador 2 (remoto)
	CanalEncerra chan bool   `json:"-"`           // Canal para forçar encerramento
}

// #############################################################################
// # ESTRUTURAS DE COMUNICAÇÃO VIA REDIS (Cliente <-> Servidor)
// #############################################################################

// --- Mensagem Genérica (Servidor -> Cliente) ---
// Toda mensagem recebida pelo cliente em seu canal pessoal terá este formato.
type RespostaGenericaCliente struct {
	Tipo string      `json:"tipo"` // "Erro", "Conexao_Sucesso", "Sorteio", "Inicio_Batalha", "Fim_Batalha", "Pedir_Carta", "Turno_Realizado"
	Data interface{} `json:"data"` // Conterá uma das structs de Resposta* abaixo
}

// --- Requisições (Cliente -> Servidor) ---

// Para o tópico global: "conectar"
type ReqConectar struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"` // Ex: "client_reply:CLIENT_ID_XYZ"
}

// Para o tópico global: "comprar_carta"
type ReqComprarCarta struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"`
}

// Para o tópico pessoal do SERVIDOR: "servidor_pessoal:SERVER_ID"
// Usada para parear, enviar mensagem, e iniciar batalha
type ReqPessoalServidor struct {
	Tipo           string `json:"tipo"` // "Parear", "Mensagem", "Batalhar"
	IdRemetente    string `json:"id_remetente"`
	CanalResposta  string `json:"canal_resposta"`
	IdDestinatario string `json:"id_destinatario,omitempty"` // Usado por "Parear" e "Mensagem"
	Mensagem       string `json:"mensagem,omitempty"`        // Usado por "Mensagem"
}

// Para o tópico pessoal do SERVIDOR: "servidor_pessoal:SERVER_ID"
// Usada para enviar a jogada (carta) durante uma batalha
type ReqJogadaBatalha struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"`
	IdBatalha     string `json:"id_batalha"`
	Carta         Tanque `json:"carta"`
}

// --- Respostas (Servidor -> Cliente) ---
// Estas structs serão encapsuladas dentro de RespostaGenericaCliente.Data

type RespostaErro struct {
	Erro string `json:"erro"`
}

// RespostaConexao envia o endereço UDP completo (host:porta)
type RespostaConexao struct {
	Mensagem             string `json:"mensagem"`
	IdServidorConectado  string `json:"id_servidor_conectado"`
	CanalPessoalServidor string `json:"canal_pessoal_servidor"` // Ex: "servidor_pessoal:SERVER_ID_123"
	CanalUDPPing         string `json:"canal_udp_ping"`         // Ex: "server1:8081" (host:porta)
}

type RespostaPareamento struct {
	Mensagem   string `json:"mensagem"` // "Pareamento realizado com JOGADOR_XYZ"
	IdParceiro string `json:"id_parceiro"`
}

type RespostaMensagem struct {
	Remetente string `json:"remetente"`
	Mensagem  string `json:"mensagem"`
}

type RespostaSorteio struct {
	Mensagem string   `json:"mensagem"`
	Cartas   []Tanque `json:"cartas"`
}

type RespostaInicioBatalha struct {
	Mensagem  string `json:"mensagem"` // "Batalha iniciada com JOGADOR_XYZ"
	IdBatalha string `json:"id_batalha"`
}

type RespostaFimBatalha struct {
	Mensagem string `json:"mensagem"` // "Batalha finalizada! Vencedor: ..."
}

type RespostaPedirCarta struct {
	Indice int `json:"indice"` // Servidor pede a carta neste índice do deck
}

type RespostaTurnoRealizado struct {
	Mensagem string   `json:"mensagem"` // "Jogador 1 jogou..."
	Cartas   []Tanque `json:"cartas"`   // Estado atual das 2 cartas na mesa
}

// #############################################################################
// # ESTRUTURAS DE COMUNICAÇÃO VIA REST (Servidor <-> Servidor)
// #############################################################################

// --- Sincronização de Estado (Líder -> Seguidores) ---

// Usada pelo Líder para notificar (POST /players/update)
type UpdatePlayerListRequest struct {
	PlayerID      string `json:"player_id"`
	ServerID      string `json:"server_id"`
	CanalResposta string `json:"canal_resposta"`
	Acao          string `json:"acao"` // "add" ou "remove"
}

// Usada pelo Líder para notificar (POST /inventory/update)
type UpdateInventoryRequest struct {
	PacotesRestantes int `json:"pacotes_restantes"`
}

// --- Requisições para o Líder (Seguidor -> Líder) ---

// Usada por um seguidor para (POST /players/connect)
type LeaderConnectRequest struct {
	PlayerID      string `json:"player_id"`
	ServerID      string `json:"server_id"` // ID do servidor que recebeu a conexão
	CanalResposta string `json:"canal_resposta"`
}

// Usada por um seguidor para (POST /cards/buy)
type LeaderBuyCardRequest struct {
	PlayerID string `json:"player_id"` // ID do jogador que está comprando
	ServerID string `json:"server_id"` // ID do servidor que atendeu o pedido
}

// --- Comunicação de Batalha (Servidor_A <-> Servidor_B) ---

// Servidor Host (J1) -> Servidor Peer (J2) (POST /battle/initiate)
// Envia o HostServidor (API do S1) para S2 saber para onde responder
type BattleInitiateRequest struct {
	IdBatalha      string `json:"id_batalha"`
	IdJogadorLocal string `json:"id_jogador_local"` // Jogador 2 (que está no Servidor B)
	IdOponente     string `json:"id_oponente"`      // Jogador 1 (que está no Servidor A)
	HostServidor   string `json:"host_servidor"`    // Endereço de API do Servidor A (ex: "server1:9090")
}

// Servidor Host (J1) -> Servidor Peer (J2) (POST /battle/request_move)
type BattleRequestMoveRequest struct {
	IdBatalha string `json:"id_batalha"`
	Indice    int    `json:"indice"`
}

// Servidor Host (J1) -> Servidor Peer (J2) (POST /battle/turn_result)
type BattleTurnResultRequest struct {
	IdBatalha string                 `json:"id_batalha"`
	Resultado RespostaTurnoRealizado `json:"resultado"`
}

// Servidor Host (J1) -> Servidor Peer (J2) (POST /battle/end)
type BattleEndRequest struct {
	IdBatalha string             `json:"id_batalha"`
	Resultado RespostaFimBatalha `json:"resultado"`
}

// Servidor Peer (J2) -> Servidor Host (J1) (POST /battle/submit_move)
// Envia a carta que J2 jogou de volta para S1
type BattleSubmitMoveRequest struct {
	IdBatalha string `json:"id_batalha"`
	Carta     Tanque `json:"carta"`
}

// --- Eleição e Health Check ---

// (GET /health)
type HealthCheckResponse struct {
	Status   string `json:"status"` // "OK"
	ServerID string `json:"server_id"`
	IsLeader bool   `json:"is_leader"`
}
