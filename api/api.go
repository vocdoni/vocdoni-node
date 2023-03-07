package api

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.8.10 fmt

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// MaxPageSize defines the maximum number of results returned by the paginated endpoints
const MaxPageSize = 10

var (
	ErrMissingModulesForHandler = fmt.Errorf("missing modules attached for enabling handler")
	ErrHandlerUnknown           = fmt.Errorf("handler unknown")
	ErrHTTPRouterIsNil          = fmt.Errorf("httprouter is nil")
	ErrBaseRouteInvalid         = fmt.Errorf("base route must start with /")
)

// API is the URL based REST API supporting bearer authentication.
type API struct {
	PrivateCalls uint64
	PublicCalls  uint64
	BaseRoute    string

	router   *httprouter.HTTProuter
	endpoint *apirest.API
	indexer  *indexer.Indexer
	vocapp   *vochain.BaseApplication
	storage  data.Storage
	//lint:ignore U1000 unused
	metricsagent *metrics.Agent
	vocinfo      *vochaininfo.VochainInfo
	censusdb     *censusdb.CensusDB
	db           db.Database // used for internal db operations
}

// NewAPI creates a new instance of the API.  Attach must be called next.
func NewAPI(router *httprouter.HTTProuter, baseRoute, dataDir string) (*API, error) {
	if router == nil {
		return nil, ErrHTTPRouterIsNil
	}
	if len(baseRoute) == 0 || baseRoute[0] != '/' {
		return nil, fmt.Errorf("%w (invalid given: %s)", ErrBaseRouteInvalid, baseRoute)
	}
	// Remove trailing slash
	if len(baseRoute) > 1 {
		baseRoute = strings.TrimSuffix(baseRoute, "/")
	}
	api := API{
		BaseRoute: baseRoute,
		router:    router,
	}
	var err error
	api.endpoint, err = apirest.NewAPI(router, baseRoute)
	if err != nil {
		return nil, err
	}
	api.db, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "db"))
	if err != nil {
		return nil, err
	}
	return &api, nil
}

// Attach takes a list of modules which are used by the handlers in order to interact with the system.
// Attach must be called before EnableHandlers.
func (a *API) Attach(vocdoniAPP *vochain.BaseApplication, vocdoniInfo *vochaininfo.VochainInfo,
	indexer *indexer.Indexer, data data.Storage, censusdb *censusdb.CensusDB) {
	a.vocapp = vocdoniAPP
	a.vocinfo = vocdoniInfo
	a.indexer = indexer
	a.storage = data
	a.censusdb = censusdb
}

// RouterHandler returns the API router handler which can be used to register new custom endpoints.
func (a *API) RouterHandler() *apirest.API {
	return a.endpoint
}

// EnableHandlers enables the list of handlers. Attach must be called before.
func (a *API) EnableHandlers(handlers ...string) error {
	for _, h := range handlers {
		switch h {
		case VoteHandler:
			if a.vocapp == nil || a.indexer == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			a.enableVoteHandlers()
		case ElectionHandler:
			if a.indexer == nil || a.vocinfo == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			a.enableElectionHandlers()
		case ChainHandler:
			if a.indexer == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			a.enableChainHandlers()
		case WalletHandler:
			if a.vocapp == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			a.enableWalletHandlers()
		case AccountHandler:
			if a.vocapp == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			a.enableAccountHandlers()
		case CensusHandler:
			a.enableCensusHandlers()
			if a.censusdb == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}

		default:
			return fmt.Errorf("%w: %s", ErrHandlerUnknown, h)
		}
	}
	return nil
}
