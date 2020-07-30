package router

import (
	"gitlab.com/vocdoni/go-dvote/log"
)

func (r *Router) censusLocal(request routerRequest) {
	auth := request.authenticated
	addr := request.address
	log.Debugf("client authorization %t. Recovered address is [%s]", auth, addr)
	if auth {
		if len(addr) < 20 {
			r.sendError(request, "cannot recover address")
			return
		}
	}
	resp := r.census.Handler(&request.MetaRequest, auth, "0x"+addr+"/")
	if !resp.Ok {
		r.sendError(request, resp.Message)
		return
	}
	request.Send(r.buildReply(request, resp))
}
