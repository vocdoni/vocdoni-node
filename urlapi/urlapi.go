package urlapi

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// MaxPageSize defines the maximum number of results returned by the paginated endpoints
const MaxPageSize = 10

// URLAPI is the URL based REST API supporting bearer authentication.
type URLAPI struct {
	PrivateCalls uint64
	PublicCalls  uint64
	BaseRoute    string

	router      *httprouter.HTTProuter
	api         *bearerstdapi.BearerStandardAPI
	scrutinizer *scrutinizer.Scrutinizer
	vocapp      *vochain.BaseApplication
	storage     data.Storage
	//lint:ignore U1000 unused
	metricsagent *metrics.Agent
	vocinfo      *vochaininfo.VochainInfo
	censusMap    sync.Map

	db db.Database
}

// NewURLAPI creates a new instance of the URLAPI.  Attach must be called next.
func NewURLAPI(router *httprouter.HTTProuter, baseRoute, dataDir string) (*URLAPI, error) {
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
	// Create local key value database
	urlapi.db, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "urlapi"))
	if err != nil {
		return nil, err
	}

	return &urlapi, nil
}

// Attach takes a list of modules which are used by the handlers in order to interact with the system.
// Attach must be called before EnableHandlers.
func (u *URLAPI) Attach(vocdoniAPP *vochain.BaseApplication, vocdoniInfo *vochaininfo.VochainInfo,
	scrutinizer *scrutinizer.Scrutinizer, data data.Storage) {
	u.vocapp = vocdoniAPP
	u.vocinfo = vocdoniInfo
	u.scrutinizer = scrutinizer
	u.storage = data
}

// EnableHandlers enables the list of handlers. Attach must be called before.
func (u *URLAPI) EnableHandlers(handlers ...string) error {
	for _, h := range handlers {
		switch h {
		case VoteHandler:
			if u.vocapp == nil || u.scrutinizer == nil {
				return fmt.Errorf("missing modules attached for enabling vote handler")
			}
			u.enableVoteHandlers()
		case ElectionHandler:
			if u.scrutinizer == nil || u.vocinfo == nil || u.storage == nil {
				return fmt.Errorf("missing modules attached for enabling election handler")
			}
			u.enableElectionHandlers()
		case ChainHandler:
			if u.scrutinizer == nil {
				return fmt.Errorf("missing modules attached for enabling chain handler")
			}
			u.enableChainHandlers()
		case WalletHandler:
			if u.vocapp == nil {
				return fmt.Errorf("missing modules attached for enabling wallet handler")
			}
			u.enableWalletHandlers()
		case AccountHandler:
			if u.vocapp == nil {
				return fmt.Errorf("missing modules attached for enabling account handler")
			}
			u.enableAccountHandlers()
		case CensusHandler:
			if u.storage == nil {
				return fmt.Errorf("missing modules attached for enabling census handler")
			}
			u.enableCensusHandlers()
		default:
			return fmt.Errorf("handler unknown %s", h)
		}
	}
	return nil
}
