package rpcapi

import (
	"context"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/util"
)

func (a *RPCAPI) censusLocal(request *api.APIrequest) (*api.APIresponse, error) {
	addr := request.Address()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return a.census.Handler(ctx, request, util.TrimHex(addr.String())+"/")
}
