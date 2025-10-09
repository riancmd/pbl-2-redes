package main

import (
	"bytes"
	"card-game/models"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// PurchaseResponse define a estrutura da resposta JSON vinda do servidor.
type PurchaseResponse struct {
	Status string      `json:"status"`
	Carta  models.Card `json:"carta"`
}

func main() {
	// peers contém os endereços de todos os servidores conhecidos no cluster.
	// O cliente tentará se conectar a eles em ordem.
	peers := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}

	log.Println("Cliente iniciado. Tentando realizar uma compra...")

	reqPayload := models.ClientPurchaseRequest{ClientID: "jogador-de-feira-075"}
	jsonData, _ := json.Marshal(reqPayload)

	var success bool = false

	for _, peerAddr := range peers {
		log.Printf("Tentando conectar ao servidor: %s", peerAddr)

		resp, err := http.Post(peerAddr+"/purchase", "application/json", bytes.NewReader(jsonData))

		if err != nil {
			log.Printf("... Falha na conexão com %s. Tentando o próximo.", peerAddr)
			continue // Tenta o próximo servidor.
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var purchaseResp PurchaseResponse
		json.Unmarshal(body, &purchaseResp)

		fmt.Println("========================================")
		fmt.Printf("CONEXÃO BEM-SUCEDIDA com %s\n", peerAddr)
		fmt.Printf("Status da Compra: %s\n", purchaseResp.Status)

		if resp.StatusCode == http.StatusOK {
			fmt.Println("--- CARTA RECEBIDA ---")
			fmt.Printf("  Nome: %s\n", purchaseResp.Carta.Nome)
			fmt.Printf("  Poder: %d\n", purchaseResp.Carta.Poder)
			fmt.Printf("  Raridade: %s\n", purchaseResp.Carta.Raridade)
		}
		fmt.Println("========================================")

		success = true
		break // Encerra o loop, pois a requisição foi processada.
	}

	if !success {
		log.Println("****************************************")
		log.Println("ERRO CRÍTICO: Não foi possível conectar a NENHUM servidor da lista.")
		log.Println("****************************************")
	}
}
