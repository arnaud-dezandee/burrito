package api

import (
	"github.com/padok-team/burrito/internal/burrito/config"
	datastore "github.com/padok-team/burrito/internal/datastore/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type API struct {
	config       *config.Config
	Client       client.Client
	Datastore    datastore.Client
	watchManager *WatchManager
}

func New(c *config.Config) *API {
	return &API{
		config: c,
	}
}

func (a *API) SetWatchManager(wm *WatchManager) {
	a.watchManager = wm
}
