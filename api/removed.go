package api

import (
	"fmt"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
)

const (
	reasonCensusIPFS = "the census IPFS routes are removed, use GET /censuses/export and POST /censuses/import"
	reasonAnonymous  = "anonymous voting is not supported"
	reasonWallet     = "the wallet API is not available"
	reasonExport     = "exporting the indexer is not available"
)

// removedRoute is a vocdoni-node API route this node does not serve.
type removedRoute struct {
	method string
	path   string
	reason string
}

// removedRoutes stay registered so that a client calling one of them gets
// ErrEndpointRemoved with the reason, instead of an empty 405 or the error of
// another route whose URL parameter happens to match the path.
var removedRoutes = []removedRoute{
	{"GET", "/accounts/count", "use GET /accounts and its pagination.totalItems"},
	{"GET", "/accounts/page/{page}", "use GET /accounts?page={page}"},
	{"GET", "/accounts/{accountId}/fees/page/{page}", "use GET /chain/fees?accountId={accountId}&page={page}"},
	{"GET", "/accounts/{accountId}/transfers/count", "use transfersCount from GET /accounts/{accountId}"},
	{"GET", "/accounts/{accountId}/transfers/page/{page}", "use GET /chain/transfers?accountId={accountId}&page={page}"},
	{"GET", "/accounts/{organizationId}/elections/count", "use GET /elections?organizationId={organizationId} and its pagination.totalItems"},
	{"GET", "/accounts/{organizationId}/elections/page/{page}", "use GET /elections?organizationId={organizationId}&page={page}"},
	{"GET", "/accounts/{organizationId}/elections/status/{status}/page/{page}", "use GET /elections?organizationId={organizationId}&status={status}&page={page}"},
	{"GET", "/censuses/export/ipfs", reasonCensusIPFS},
	{"GET", "/censuses/export/ipfs/list", reasonCensusIPFS},
	{"GET", "/censuses/import/{ipfscid}", reasonCensusIPFS},
	{"GET", "/chain/blocks/{height}/transactions/page/{page}", "use GET /chain/transactions?height={height}&page={page}"},
	{"GET", "/chain/export/indexer", reasonExport},
	{"GET", "/chain/fees/page/{page}", "use GET /chain/fees?page={page}"},
	{"GET", "/chain/fees/reference/{reference}/page/{page}", "use GET /chain/fees?reference={reference}&page={page}"},
	{"GET", "/chain/fees/type/{type}/page/{page}", "use GET /chain/fees?type={type}&page={page}"},
	{"GET", "/chain/info/circuit", reasonAnonymous},
	{"GET", "/chain/organizations/count", "use organizationCount from GET /chain/info"},
	{"POST", "/chain/organizations/filter/page/{page}", "use GET /chain/organizations?organizationId={id}&page={page}"},
	{"GET", "/chain/organizations/page/{page}", "use GET /chain/organizations?page={page}"},
	{"GET", "/chain/transactions/count", "use transactionCount from GET /chain/info"},
	{"GET", "/chain/transactions/page/{page}", "use GET /chain/transactions?page={page}"},
	{"GET", "/chain/transactions/{height}/{index}", "use GET /chain/transactions?height={height}, whose items carry index and hash, then GET /chain/transactions/{hash}"},
	{"POST", "/elections/filter", "use GET /elections with query params"},
	{"POST", "/elections/filter/page/{page}", "use GET /elections with query params and page={page}"},
	{"GET", "/elections/page/{page}", "use GET /elections?page={page}"},
	{"GET", "/elections/{electionId}/votes/count", "use voteCount from GET /elections/{electionId}"},
	{"GET", "/elections/{electionId}/votes/page/{page}", "use GET /votes?electionId={electionId}&page={page}"},
	{"GET", "/siks/proof/{address}", reasonAnonymous},
	{"GET", "/siks/roots", reasonAnonymous},
	{"GET", "/siks/{address}", reasonAnonymous},
	{"GET", "/wallet/bootstrap", reasonWallet},
	{"POST", "/wallet/election", reasonWallet},
	{"GET", "/wallet/transfer/{dstAddress}/{amount}", reasonWallet},
}

// enableRemovedRoutes registers every removed route with a handler that
// returns ErrEndpointRemoved.
func (a *API) enableRemovedRoutes() error {
	for _, r := range removedRoutes {
		msg := fmt.Sprintf("%s %s: %s", r.method, r.path, r.reason)
		if err := a.Endpoint.RegisterMethod(r.path, r.method, apirest.MethodAccessTypePublic,
			func(*apirest.APIdata, *httprouter.HTTPContext) error {
				return ErrEndpointRemoved.With(msg)
			},
		); err != nil {
			return err
		}
	}
	return nil
}
