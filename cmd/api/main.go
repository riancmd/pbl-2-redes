package main

import (
	"pbl-2-redes/internal/handlers"
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/usecases"
)

// cadastrar e listar usuários

func main() {
	repos := repositories.New()
	useCases := usecases.New(repos)

	err := useCases.AddCardsFromFile("../../internal/data/cardVault.json", 100000)

	if err != nil {
		panic(err)
	}

	h := handlers.New(useCases)

	//port, err := strconv.Atoi(os.Args[1])

	//if err != nil {
	//	h.Listen(7777)
	//}

	//h.Listen(port) // roda na porta x
	h.Listen(7777)
}
