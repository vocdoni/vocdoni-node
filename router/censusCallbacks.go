package router

import (
	"context"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
)

func (r *Router) censusLocal(request routerRequest) {
	auth := request.authenticated
	addr := request.address
	log.Debugf("client authorization %t. Recovered address is [%s]", auth, addr.Hex())
	if auth {
		if len(addr) < 20 {
			r.sendError(request, "cannot recover address")
			return
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := r.census.Handler(ctx, &request.MetaRequest, auth, util.TrimHex(addr.String())+"/")
	if !resp.Ok {
		r.sendError(request, resp.Message)
		return
	}
	request.Send(r.buildReply(request, resp))
}
