package router

import (
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func (r *Router) censusLocal(request routerRequest) {
	var cresponse *types.MetaResponse
	auth := request.authenticated
	addr := request.address
	log.Debugf("client authorization %t. Recovered address is [%s]", auth, addr)
	if auth {
		if len(addr) < signature.AddressLength {
			r.sendError(request, "cannot recover address")
			return
		}
	}
	cresponse = r.census.Handler(&request.MetaRequest, auth, "0x"+addr+"/")
	if !cresponse.Ok {
		r.sendError(request, cresponse.Message)
		return
	}
	var response types.ResponseMessage
	response.MetaResponse = *cresponse
	r.transport.Send(r.buildReply(request, response))
}
