package router

import (
	"encoding/json"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	vochain "gitlab.com/vocdoni/go-dvote/vochain/types"
)

func submitEnvelope(request routerRequest, router *Router) {
	voteTxArgs := new(vochain.VoteTxArgs)
	voteTxArgs.ProcessID = request.structured.ProcessId
	voteTxArgs.Nullifier = request.structured.Nullifier
	voteTxArgs.VotePackage = request.structured.Payload

	voteTxBytes := []byte(voteTxArgs.String())

	req := abci.RequestDeliverTx{Tx: voteTxBytes}
	vochainReqRes := router.vochainClient.DeliverTxAsync(req)

	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())

	vochainResponse := vochainReqRes.Response.GetDeliverTx()
	if vochainResponse.Code != 0 {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
	}

	var err error
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err.Error())
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling submitEnvelope reply: %s", err)
	}

	router.transport.Send(buildReply(request.context, rawApiResponse))

}

func getEnvelopeStatus(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// request.structured.Nullifier
	// getEnvelopeStatus
}

func getEnvelope(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// request.structured.Nullifier
	// getEnvelope
}

func getEnvelopeHeight(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// getEnvelopeHeight
}

func getBlockHeight(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// getBlockHeight
}

func getProcessList(request routerRequest, router *Router) {

	// request.structured.From
	// request.structured.ListSize
	// getProcessList
}

func getEnvelopeList(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// request.structured.From
	// request.structured.ListSize
	// getEnvelopeList
}
