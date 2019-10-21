package router

import (
	"encoding/json"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	vochain "gitlab.com/vocdoni/go-dvote/types"
)

func submitEnvelope(request routerRequest, router *Router) {
	voteTxArgs := new(vochain.VoteTx)
	voteTxArgs.ProcessID = request.structured.ProcessId
	voteTxArgs.Nonce = request.structured.Nonce
	voteTxArgs.Nullifier = request.structured.Nullifier
	voteTxArgs.VotePackage = request.structured.Payload
	voteTxArgs.Proof = request.structured.ProofData
	voteTxArgs.Type = "vote"
	voteTxArgs.Signature = request.structured.Signature

	voteTxBytes, err := json.Marshal(voteTxArgs)
	if err != nil {
		log.Errorf("error marshaling voteTx args: %s", err.Error())
	}

	res, err := router.tmclient.BroadcastTxSync(voteTxBytes)
	if err != nil {
		log.Warnf("cannot broadcast tx: %s", err)
	} else {
		log.Infof("transactions result details: %s", res.Data.String())
	}

	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())

	// not sure if its enough information
	if res.Code != 0 {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
	}
	apiResponse.Response.Message = res.Data.String()

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
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())
	qdata := vochain.QueryData{
		Method:    "getEnvelopeHeight",
		ProcessID: request.structured.ProcessId,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err.Error())
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
	}
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Height)
	if apiResponse.Response.Height == 0 {
		apiResponse.Response.Height = -1
	}
	log.Debugf("Response height is: %d", apiResponse.Response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err.Error())
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err.Error())
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelopeHeight reply: %s", err)
	}

	log.Debugf("api response: %+v", apiResponse.Response)
	router.transport.Send(buildReply(request.context, rawApiResponse))

}

func getBlockHeight(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())
	qdata := vochain.QueryData{
		Method: "getBlockHeight",
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err.Error())
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
	}
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Height)
	log.Debugf("Response height is: %d", apiResponse.Response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err.Error())
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err.Error())
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getBlockHeight reply: %s", err)
	}
	log.Debugf("api response: %+v", apiResponse.Response)
	router.transport.Send(buildReply(request.context, rawApiResponse))
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
