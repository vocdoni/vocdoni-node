package router

import (
	"encoding/json"
	"fmt"

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func censusLocal(request routerRequest, router *Router) {
	var response types.ResponseMessage
	var err error
	auth := request.authenticated
	addr := request.address
	log.Debugf("client authorization %t. Recovered address is [%s]", auth, addr)
	if auth && len(addr) < signature.AddressLength {
		sendError(router.transport, router.signer, request.context, request.id, "cannot recover address")
		return
	}
	cresponse := router.census.Handler(&request.structured, auth, "0x"+addr+"/")
	if cresponse.Ok != true {
		sendError(router.transport, router.signer, request.context, request.id, cresponse.Error)
		return
	}
	response.Response = *cresponse
	response.ID = request.id
	response.Response.Request = request.id
	response.Signature, err = router.signer.SignJSON(response.Response)
	if err != nil {
		log.Warn(err)
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("could not unmarshal response (%s)", err))
	} else {
		log.Infof("sending census resposne: %s", rawResponse)
		router.transport.Send(buildReply(request.context, rawResponse))
	}
}
