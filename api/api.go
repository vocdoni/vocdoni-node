package api

////// Disabled autoswag due to https://github.com/swaggo/swag/issues/1267
////// TODO: re-enable when a fixed swaggo/swag is released
////// and remove the workaround done by @selankon on docs/models/models.go
////go:generate go run go.vocdoni.io/dvote/api/autoswag
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.8.10 fmt

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

//	@title			Vocdoni API
//	@version		2.0.0
//	@description	Vocdoni API is a REST API
//	@description.markdown

//	@tag.name			Chain
//	@tag.description	Everything about internal Vochain information (transactions, organizations, blocks, stats...)
//	@tag.name			Accounts
//	@tag.description	Related to account metadata (additional account information like balance, storage URI, process count...)
//	@tag.name			Elections
//	@tag.description	Create, manage and get information about elections
//	@tag.name			Censuses
//	@tag.description	Manage census: create, add, get, verify...
//	@tag.name			Votes
//	@tag.description	Submit, get, and verify votes
//	@tag.name			Wallet
//	@tag.description	Operations for the wallets managed backend side
//	@tag.name			SIK
//	@tag.description	The Secret Identity Key (SIK) operations used on anonymous voting

//	@schemes	https
//	@host		api-dev.vocdoni.net
//	@BasePath	/v2

//	@securityDefinitions.basic	BasicAuth

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
	Endpoint *apirest.API

	indexer  *indexer.Indexer
	vocapp   *vochain.BaseApplication
	storage  data.Storage
	vocinfo  *vochaininfo.VochainInfo
	censusdb *censusdb.CensusDB
	db       db.Database // used for internal db operations

	censusPublishStatusMap sync.Map // used to store the status of the census publishing process when async
}

// NewAPI creates a new instance of the API.  Attach must be called next.
func NewAPI(router *httprouter.HTTProuter, baseRoute, dataDir, dbType string) (*API, error) {
	if router == nil {
		return nil, ErrHTTPRouterIsNil
	}
	if dbType == "" {
		dbType = db.TypePebble
	}
	if len(baseRoute) == 0 || baseRoute[0] != '/' {
		return nil, fmt.Errorf("%w (invalid given: %s)", ErrBaseRouteInvalid, baseRoute)
	}
	// Remove trailing slash
	if len(baseRoute) > 1 {
		baseRoute = strings.TrimSuffix(baseRoute, "/")
	}
	api := API{}
	var err error
	api.Endpoint, err = apirest.NewAPI(router, baseRoute)
	if err != nil {
		return nil, err
	}
	mdb, err := metadb.New(dbType, filepath.Join(dataDir, "db"))
	if err != nil {
		return nil, err
	}
	api.db = prefixeddb.NewPrefixedDatabase(mdb, []byte("api/"))
	return &api, nil
}

// Attach takes a list of modules which are used by the handlers in order to interact with the system.
// Attach must be called before EnableHandlers.
func (a *API) Attach(vocdoniAPP *vochain.BaseApplication, vocdoniInfo *vochaininfo.VochainInfo,
	indexer *indexer.Indexer, data data.Storage, censusdb *censusdb.CensusDB,
) {
	a.vocapp = vocdoniAPP
	a.vocinfo = vocdoniInfo
	a.indexer = indexer
	a.storage = data
	a.censusdb = censusdb
}

// EnableHandlers enables the list of handlers. Attach must be called before.
func (a *API) EnableHandlers(handlers ...string) error {
	for _, h := range handlers {
		switch h {
		case VoteHandler:
			if a.vocapp == nil || a.indexer == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			if err := a.enableVoteHandlers(); err != nil {
				return err
			}
		case ElectionHandler:
			if a.indexer == nil || a.vocinfo == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			if err := a.enableElectionHandlers(); err != nil {
				return err
			}
		case ChainHandler:
			if a.indexer == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			if err := a.enableChainHandlers(); err != nil {
				return err
			}
		case WalletHandler:
			if a.vocapp == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			if err := a.enableWalletHandlers(); err != nil {
				return err
			}
		case AccountHandler:
			if a.vocapp == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
			if err := a.enableAccountHandlers(); err != nil {
				return err
			}
		case CensusHandler:
			if err := a.enableCensusHandlers(); err != nil {
				return err
			}
			if a.censusdb == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}
		case SIKHandler:
			if err := a.enableSIKHandlers(); err != nil {
				return err
			}
			if a.censusdb == nil {
				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
			}

		default:
			return fmt.Errorf("%w: %s", ErrHandlerUnknown, h)
		}
	}
	return nil
}
