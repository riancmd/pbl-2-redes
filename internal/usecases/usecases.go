package usecases

import (
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/utils"
)

type UseCases struct {
	repos *repositories.Repositories
	utils *utils.Utils
}

func New(repos *repositories.Repositories) *UseCases {
	return &UseCases{
		repos: repos,
		utils: utils.New(),
	}
}
