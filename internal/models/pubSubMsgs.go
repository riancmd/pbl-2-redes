package models

import (
	"encoding/json"
)

// Resposta externa para o canal de resposta do cliente (dentro possui resposta especifica)
type ExternalResponse struct {
	Type   string          `json:"type"`
	UserId string          `json:"userId"`
	Data   json.RawMessage `json:"data"` //Struct especifica para ser decodificada
}

// Requisição externa para canal do servidor que lidará com requisições envolvendo pareamento (batalha ou troca)
type ExternalRequest struct {
	Type   string          `json:"type"`
	UserId string          `json:"userId"`
	Data   json.RawMessage `json:"data"` //Struct especifica para ser decodificada
}

// Requisição de login/cadastro
type AuthenticationRequest struct {
	Type               string `json:"type"` //login ou register
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	Username           string `json:"username"`
	Password           string `json:"password"`
}

// Requisição de compra de pacote
type PurchaseRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
}

// Requisição de entrar em batalha
type MatchRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
}

// "Requisição" de envio de nova carta
type NewCardRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	Card               Card   `json:"card"`
}

// Requisição de Troca de carta
type TradeRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
}

// "Requisição" de envio de uma ação em uma batalha
type GameActionRequest struct {
	Type               string `json:"type"`
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"`
}

// Resposta de Login/Cadastro
type AuthResponse struct {
	Status        bool   `json:"status"`
	Username      string `json:"username"`
	UDPPort       string `json:"udpPort"`
	ServerChannel string `json:"serverChannel"`
	Message       string `json:"message"`
}

// Resposta para batalhas (entrada ou não / solicitação de envio de nova carta)
type MatchResponse struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// Resposta de compra do cliente
type ClientPurchaseResponse struct {
	Status           bool    `json:"status"`
	Message          string  `json:"message"`
	BoosterGenerated Booster `json:"boosterGenerated"`
}

// Resposta para trocas de cartas (pareamento ou não / solicitação de envio de nova carta)
type TradeResponse struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// Payload para ações em batalha
type PayLoad struct {
	P1 *User `json:"p1"`
	P2 *User `json:"p2"`

	Info        string                `json:"info"`
	Turn        string                `json:"turn"`
	Hand        []Card                `json:"hand"`
	Sanity      map[string]int        `json:"sanity"`
	DreamStates map[string]DreamState `json:"dreamStates"`
	Round       int                   `json:"round"`
}

// Resposta de erro
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
