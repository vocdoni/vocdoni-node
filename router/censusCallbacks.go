package router

import (
	"context"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp := r.census.Handler(ctx, &request.MetaRequest, auth, util.TrimHex(addr.String())+"/")
	if !resp.Ok {
		r.sendError(request, resp.Message)
		return
	}
	if err := request.Send(r.buildReply(request, resp)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}
