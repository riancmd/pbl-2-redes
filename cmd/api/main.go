package main

import (
	"os"
	"pbl-2-redes/internal/handlers"
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/usecases"
	"strconv"
)

// cadastrar e listar usuários

func main() {
	repos := repositories.New()
	useCases := usecases.New(repos)

	h := handlers.New(useCases)

	port, err := strconv.Atoi(os.Args[1])

	if err != nil {
		h.Listen(7777)
	}

	h.Listen(port) // roda na porta x
}
