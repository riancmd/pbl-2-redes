package models

// pacote com os modelos principais utilizados ao longo do projeto

// estruturas do jogo
// a struct da nossa carta (o tanque)
type Tanque struct {
	Modelo     string `json:"modelo"`
	Id_jogador string `json:"id_jogador"`
	Vida       int    `json:"vida"`
	Ataque     int    `json:"ataque"`
}

// Batalha: isso aqui fica no map s.batalhas la do server
type Batalha struct {
	Jogador1     string      `json:"jogador1"`
	Jogador2     string      `json:"jogador2"`
	ServidorJ1   string      `json:"servidor_j1"` // ex: "server1:9090"
	ServidorJ2   string      `json:"servidor_j2"` // ex: "server2:9091"
	CanalJ1      chan Tanque `json:"-"`           // canal pra receber a carta do j1 (q ta no mesmo server)
	CanalJ2      chan Tanque `json:"-"`           // canal pra receber a carta do j2 (q vem pela api)
	CanalEncerra chan bool   `json:"-"`           // pra gnt mandar a goroutine da batalha parar
}

// Troca: mesma logica da batalha, so q pra troca
// fica no map s.trades do server
type Troca struct {
	Jogador1     string      `json:"jogador1"`
	Jogador2     string      `json:"jogador2"`
	ServidorJ1   string      `json:"servidor_j1"`
	ServidorJ2   string      `json:"servidor_j2"`
	CanalJ1      chan Tanque `json:"-"` // canal pra receber a carta do j1
	CanalJ2      chan Tanque `json:"-"` // canal pra receber a carta do j2
	CanalEncerra chan bool   `json:"-"` // pra forçar o encerramento
}

// comunicacao via redis (cliente <-> servidor)

// msg generica q o servidor manda pro cliente
// o cliente sempre recebe isso e tem q olhar o 'Tipo' pra saber oq é
type RespostaGenericaCliente struct {
	Tipo string      `json:"tipo"` // "Erro", "Conexao_Sucesso", "Sorteio", "Inicio_Batalha", etc
	Data interface{} `json:"data"` // aqui vai a struct especifica (RespostaConexao, RespostaErro, etc)
}

// reqs do cliente pro servidor

// qnd o cliente abre o jogo, ele manda isso pro topico 'conectar'
type ReqConectar struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"` // ex: "client_reply:UUID_DO_CLIENTE"
}

// qnd o cliente quer comprar carta, manda isso pro topico 'comprar_carta'
type ReqComprarCarta struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"`
}

// req pro canal pessoal do servidor (parear, msg, iniciar batalha/troca)
type ReqPessoalServidor struct {
	Tipo           string `json:"tipo"` // "Parear", "Mensagem", "Batalhar", "Trocar"
	IdRemetente    string `json:"id_remetente"`
	CanalResposta  string `json:"canal_resposta"`
	IdDestinatario string `json:"id_destinatario,omitempty"` // pra quem eh
	Mensagem       string `json:"mensagem,omitempty"`        // se for tipo "Mensagem"
}

// qnd o server pede nossa carta da batalha, a gnt manda isso
type ReqJogadaBatalha struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"`
	IdBatalha     string `json:"id_batalha"`
	Carta         Tanque `json:"carta"`
}

// qnd a gnt oferta uma carta na troca, manda isso
type ReqCartaTroca struct {
	IdRemetente   string `json:"id_remetente"`
	CanalResposta string `json:"canal_resposta"`
	IdTroca       string `json:"id_troca"`
	Carta         Tanque `json:"carta"` // a carta q o jogador ta ofertando
}

// respostas do servidor pro cliente
// (elas vao dentro do campo 'Data' da RespostaGenericaCliente)

type RespostaErro struct {
	Erro string `json:"erro"`
}

// qnd a conexao da certo
type RespostaConexao struct {
	Mensagem             string `json:"mensagem"`
	IdServidorConectado  string `json:"id_servidor_conectado"`
	CanalPessoalServidor string `json:"canal_pessoal_servidor"` // ex: "servidor_pessoal:server1"
	CanalUDPPing         string `json:"canal_udp_ping"`         // ex: "server1:8081" (host:porta) pro heartbeat
}

type RespostaPareamento struct {
	Mensagem   string `json:"mensagem"` // "pareamento realizado com..."
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
	Mensagem  string `json:"mensagem"` // "batalha iniciada com..."
	IdBatalha string `json:"id_batalha"`
}

type RespostaFimBatalha struct {
	Mensagem string `json:"mensagem"` // "batalha finalizada! vencedor: ..."
}

type RespostaPedirCarta struct {
	Indice int `json:"indice"` // server pedindo a carta da batalha (ele fala o *indice* q ele quer)
}

type RespostaTurnoRealizado struct {
	Mensagem string   `json:"mensagem"` // "jogador 1 jogou..."
	Cartas   []Tanque `json:"cartas"`   // estado atual das 2 cartas na mesa
}

type RespostaInicioTroca struct {
	Mensagem string `json:"mensagem"` // "troca iniciada com..."
	IdTroca  string `json:"id_troca"`
}

// server (local ou remoto) avisando q é nossa vez de ofertar na troca
type RespostaPedirCartaTroca struct {
	IdTroca string `json:"id_troca"` // so pra gnt saber pra qual troca eh
}

// o resultado final da troca. se 'CartaRecebida' tiver vazia, falhou
type RespostaResultadoTroca struct {
	Mensagem      string `json:"mensagem"`       // "troca realizada com sucesso!"
	CartaRecebida Tanque `json:"carta_recebida"` // a carta q o jogador recebeu
}

// comunicacao via rest (servidor <-> servidor)

// sync de estado (lider manda pros seguidores)

// lider avisando q um player entrou ou saiu (POST /players/update)
type UpdatePlayerListRequest struct {
	PlayerID      string `json:"player_id"`
	ServerID      string `json:"server_id"`
	CanalResposta string `json:"canal_resposta"`
	Acao          string `json:"acao"` // "add" ou "remove"
}

// lider avisando q o estoque de pacotes mudou (POST /inventory/update)
type UpdateInventoryRequest struct {
	PacotesRestantes int `json:"pacotes_restantes"`
}

// reqs dos seguidores pro lider

// seguidor avisando o lider q um player novo conectou nele (POST /players/connect)
type LeaderConnectRequest struct {
	PlayerID      string `json:"player_id"`
	ServerID      string `json:"server_id"` // id do server q recebeu a conexao
	CanalResposta string `json:"canal_resposta"`
}

// seguidor pedindo pro lider processar uma compra (POST /cards/buy)
type LeaderBuyCardRequest struct {
	PlayerID string `json:"player_id"` // id do jogador q ta comprando
	ServerID string `json:"server_id"` // id do server q atendeu o pedido
}

// comunicacao da batalha (s1 <-> s2)

// s1 (host) -> s2 (peer) pra iniciar a batalha (POST /battle/initiate)
// manda o 'HostServidor' pra s2 saber pra qm responder
type BattleInitiateRequest struct {
	IdBatalha      string `json:"id_batalha"`
	IdJogadorLocal string `json:"id_jogador_local"` // jogador 2 (q ta no s2)
	IdOponente     string `json:"id_oponente"`      // jogador 1 (q ta no s1)
	HostServidor   string `json:"host_servidor"`    // api do s1 (ex: "server1:9090")
}

// s1 (host) -> s2 (peer) pra pedir a carta do j2 (POST /battle/request_move)
type BattleRequestMoveRequest struct {
	IdBatalha string `json:"id_batalha"`
	Indice    int    `json:"indice"`
}

// s1 (host) -> s2 (peer) pra mandar o resultado do turno (POST /battle/turn_result)
type BattleTurnResultRequest struct {
	IdBatalha string                 `json:"id_batalha"`
	Resultado RespostaTurnoRealizado `json:"resultado"`
}

// s1 (host) -> s2 (peer) pra avisar q a batalha acabou (POST /battle/end)
type BattleEndRequest struct {
	IdBatalha string             `json:"id_batalha"`
	Resultado RespostaFimBatalha `json:"resultado"`
}

// s2 (peer) -> s1 (host) pra mandar a carta q o j2 jogou (POST /battle/submit_move)
type BattleSubmitMoveRequest struct {
	IdBatalha string `json:"id_batalha"`
	Carta     Tanque `json:"carta"`
}

// comunicacao da troca (s1 <-> s2)

// s1 (host) -> s2 (peer) pra iniciar a troca (POST /trade/initiate)
type TradeInitiateRequest struct {
	IdTroca        string `json:"id_troca"`
	IdJogadorLocal string `json:"id_jogador_local"` // jogador 2
	IdOponente     string `json:"id_oponente"`      // jogador 1
	HostServidor   string `json:"host_servidor"`    // api do s1
}

// s1 (host) -> s2 (peer) pra pedir a carta do j2 (POST /trade/request_card)
type TradeRequestCardRequest struct {
	IdTroca string `json:"id_troca"`
}

// s1 (host) -> s2 (peer) pra mandar o resultado final da troca (POST /trade/result)
type TradeResultRequest struct {
	IdTroca       string `json:"id_troca"`
	CartaRecebida Tanque `json:"carta_recebida"` // a carta q o j1 ofertou (e q o j2 vai receber)
}

// s2 (peer) -> s1 (host) pra mandar a carta q o j2 ofertou (POST /trade/submit_card)
type TradeSubmitCardRequest struct {
	IdTroca string `json:"id_troca"`
	Carta   Tanque `json:"carta"` // a carta q o j2 ta ofertando
}

// eleicao e health check

// (GET /health)
type HealthCheckResponse struct {
	Status   string `json:"status"` // "OK"
	ServerID string `json:"server_id"`
	IsLeader bool   `json:"is_leader"`
}
