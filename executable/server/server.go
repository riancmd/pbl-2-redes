package main

import (
	"card-game/handlers"
	"card-game/models"
	"card-game/router"
	"card-game/server"
	"log"
)

func main() {
	// port define a porta de escuta para esta instância do servidor.
	// Altere este valor para iniciar instâncias diferentes na mesma máquina.
	port := "8081"

	// peers contém a lista de todos os servidores conhecidos no cluster.
	peers := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}
	myAddress := "http://localhost:" + port

	// cardPool define o conjunto completo de cartas que podem ser sorteadas por um jogador.
	cardPool := []models.Card{
		{ID: "c001", Nome: "Dragão das Chamas Azuis", Poder: 3000, Raridade: "Ultra Rara"},
		{ID: "c002", Nome: "Elfo Arqueiro", Poder: 1200, Raridade: "Comum"},
		{ID: "c003", Nome: "Golem de Pedra", Poder: 2100, Raridade: "Rara"},
		{ID: "c004", Nome: "Fada Curandeira", Poder: 800, Raridade: "Comum"},
		{ID: "c005", Nome: "Espectro das Sombras", Poder: 1900, Raridade: "Super Rara"},
		{ID: "c006", Nome: "Cavaleiro Valente", Poder: 1700, Raridade: "Comum"},
	}

	log.Printf("Iniciando servidor em %s", myAddress)

	serverCore := server.NewServer(peers, myAddress, 5, cardPool) // Cada servidor começa com 5 pacotes.
	apiHandlers := handlers.NewHandler(serverCore)
	ginRouter := router.SetupRouter(apiHandlers)

	if err := ginRouter.Run(":" + port); err != nil {
		log.Fatalf("Falha ao iniciar servidor Gin: %v", err)
	}
}
