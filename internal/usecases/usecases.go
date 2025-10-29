package usecases

import (
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/utils"
	"sync"
)

type UseCases struct {
	repos     *repositories.Repositories
	utils     *utils.Utils
	sync      ClusterSync
	matchesMU sync.Mutex
	usersMU   sync.Mutex
	cardsMU   sync.Mutex
	bqueue    sync.Mutex
	tqueue    sync.Mutex
}

func New(repos *repositories.Repositories, csync ClusterSync) *UseCases {
	return &UseCases{
		repos:     repos,
		utils:     utils.New(),
		sync:      csync,
		matchesMU: sync.Mutex{},
		usersMU:   sync.Mutex{},
		cardsMU:   sync.Mutex{},
		bqueue:    sync.Mutex{},
		tqueue:    sync.Mutex{},
	}
}
