package main

import (
	"encoding/json"
	"net"
)

// no protocolo pub-sub:
// -> o tópico vai ser o assunto das mensagens
// -> o evento é se é pra se inscrever no assunto, publicar ou desinscrever
// resto mantém o msm de antes
type Msg struct {
	Topic string          `json:"topic"`
	Event string          `json:"event"`
	UID   string          `json:"uid"`
	Data  json.RawMessage `json:"data"`
}

var (
	connection net.Conn
	// os encoders/decoders json vão continuar passando os dados json via o socket tcp
	encoder *json.Encoder
	decoder *json.Decoder

	UID string
)

func main() {
	//blaa bla bla client normal faz conexão tcp

	// fico verificando o que o servidor manda
	go handleServerMessages()
}

// originalmente existiam duas funções pra:
// 1. verificar as msgs do servidor
// 2. olhar qual o código da mensagem (usou carta x: quem? zezinho)
// a função 2 fica num switch case rodando pra ver se recebeu algum "sinal"
// agora ela vai verificar o tópico e ir mandando pra uma estrutura que represente esse tópico
// isso pode ser pensado dps, mas aí com base nesse canal de comunicação (nn necessariamente um channel)
// que a nova lógica vai funcionar
func handleServerMessages() {
	// loop que verifica mensagem do servidor via decoder
	// se tiver alguma coisa, vai chamar o handleResponse
	return
}

func handleResponse() {
	// vai ter switch case pra cada topico que for decidido
	// como dito, manda os dados pro tópico específico
	// ent algo como switch msg.Topic
	// case battleLog
	// e ai la manda tudo que for relacionado ao fluxo de batalha
	return
}

// manda o tópico e vc se inscreve nele
func subscribe(topic string) error {
	msg := Msg{
		Topic: topic,
		Event: "subscribe",
		UID:   UID,
	}
	return encoder.Encode(msg) // passo da mesma forma
}

// manda o tópico e vc se desinscreve dele
func unsubscribe(topic string) error {
	msg := Msg{
		Topic: topic,
		Event: "unsubscribe",
		UID:   UID,
	}
	return encoder.Encode(msg) // passo da mesma forma
}

// publico alguma coisa no tópico
// ou seja, aviso q "usei carta x", "desisti da partida"
func publish(topic string, data any) error {
	dados, _ := json.Marshal(data)

	msg := Msg{
		Topic: topic,
		Event: "publish",
		UID:   UID,
		Data:  dados,
	}
	return encoder.Encode(msg) // passo da mesma forma
}
