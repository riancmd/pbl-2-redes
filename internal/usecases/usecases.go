package usecases

import (
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/utils"
)

type UseCases struct {
	repos *repositories.Repositories
	utils *utils.Utils
	sync  ClusterSync
}

func New(repos *repositories.Repositories, sync ClusterSync) *UseCases {
	return &UseCases{
		repos: repos,
		utils: utils.New(),
		sync:  sync,
	}
}
