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

// Requisição de ping
type PingRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
}

// Requisição de Troca de carta
type TradeRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
}

// "Requisição" de envio de uma ação em uma batalha
type GameActionRequest struct {
	Type               string `json:"type"`
	UserId             string `json:"userid"`
	ClientReplyChannel string `json:"clientReplyChannel"`
}

// Resposta de Login/Cadastro
type AuthResponse struct {
	Status   bool   `json:"status"`
	Username string `json:"username"`
	Message  string `json:"message"`
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
	BoosterGenerated Booster `json:"booster"`
}

// Resposta de Pong
type PongResponse struct {
	Status bool `json:"status"`
}

// Resposta para trocas de cartas (pareamento ou não / solicitação de envio de nova carta)
type TradeResponse struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// Resposta de erro
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
