package urlapi

import (
	"fmt"
	"strings"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

type URLAPI struct {
	PrivateCalls uint64
	PublicCalls  uint64
	BaseRoute    string

	router      *httprouter.HTTProuter
	api         *bearerstdapi.BearerStandardAPI
	scrutinizer *scrutinizer.Scrutinizer
	vocapp      *vochain.BaseApplication
	//lint:ignore U1000 unused
	metricsagent *metrics.Agent
	vocinfo      *vochaininfo.VochainInfo
}

func NewURLAPI(router *httprouter.HTTProuter, baseRoute string) (*URLAPI, error) {
	if router == nil {
		return nil, fmt.Errorf("httprouter is nil")
	}
	if len(baseRoute) == 0 || baseRoute[0] != '/' {
		return nil, fmt.Errorf("invalid base route (%s), it must start with /", baseRoute)
	}
	// Remove trailing slash
	if len(baseRoute) > 1 {
		baseRoute = strings.TrimSuffix(baseRoute, "/")
	}
	urlapi := URLAPI{
		BaseRoute: baseRoute,
		router:    router,
	}
	var err error
	urlapi.api, err = bearerstdapi.NewBearerStandardAPI(router, baseRoute)
	if err != nil {
		return nil, err
	}

	return &urlapi, nil
}
