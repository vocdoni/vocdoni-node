package router

import (
	"context"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

func (r *Router) censusLocal(request RouterRequest) {
	auth := request.authenticated
	addr := request.address
	log.Debugf("client authorization %t. Recovered address is [%s]", auth, addr.Hex())
	if auth {
		if len(addr) < 20 {
			r.SendError(request, "cannot recover address")
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp := r.census.Handler(ctx, &request.MetaRequest, auth, util.TrimHex(addr.String())+"/")
	if !resp.Ok {
		r.SendError(request, resp.Message)
		return
	}
	if err := request.Send(r.BuildReply(request, resp)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}
