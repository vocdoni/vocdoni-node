package router

import (
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func censusLocal(request routerRequest, router *Router) {
	var cresponse *types.MetaResponse
	auth := request.authenticated
	addr := request.address
	log.Debugf("client authorization %t. Recovered address is [%s]", auth, addr)
	if auth {
		if len(addr) < signature.AddressLength {
			router.sendError(request, "cannot recover address")
			return
		}
	}
	cresponse = router.census.Handler(&request.MetaRequest, auth, "0x"+addr+"/")
	if !cresponse.Ok {
		router.sendError(request, cresponse.Message)
		return
	}
	var response types.ResponseMessage
	response.MetaResponse = *cresponse
	router.transport.Send(router.buildReply(request, response))
}
