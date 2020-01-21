package router

import (
	"encoding/json"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func submitEnvelope(request routerRequest, router *Router) {
	voteTxArgs := new(types.VoteTx)
	voteTxArgs.ProcessID = request.Payload.ProcessID
	voteTxArgs.Nonce = request.Payload.Nonce
	voteTxArgs.Nullifier = request.Payload.Nullifier
	voteTxArgs.VotePackage = request.Payload.VotePackage
	voteTxArgs.Proof = request.Payload.Proof
	voteTxArgs.Type = "vote"
	voteTxArgs.Signature = request.Payload.Signature

	voteTxBytes, err := json.Marshal(voteTxArgs)
	if err != nil {
		log.Errorf("error marshaling voteTx args: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal voteTx args")
		return
	}

	res, err := router.tmclient.BroadcastTxSync(voteTxBytes)
	if err != nil || res.Code != 0 {
		log.Warnf("cannot broadcast tx (res.Code=%d): %s || %s", res.Code, err, string(res.Data))
		sendError(router.transport, router.signer, request.context, request.id, string(res.Data))
		return
	}
	log.Infof("transaction hash: %s, transaction code: %d", res.Hash, res.Code)

	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = (res.Code == 0)

	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling submitEnvelope reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelopeStatus(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	apiResponse.Registered = types.True
	qdata := types.QueryData{
		Method:    "getEnvelopeStatus",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal query data")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot query")
		return
	}
	if queryResult.Response.Code != 0 {
		apiResponse.Registered = types.False
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getEnvelopeStatus reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}

	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelope(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	qdata := types.QueryData{
		Method:    "getEnvelope",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal query data")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot query")
		return
	}
	if queryResult.Response.Code != 0 {
		sendError(router.transport, router.signer, request.context, request.id, queryResult.Response.GetInfo())
		return
	}
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Payload)
	if err != nil {
		log.Errorf("cannot unmarshal vote package: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot unmarshal vote package")
		return
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getEnvelope reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}

	log.Debugf("getEnvelope response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelopeHeight(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	qdata := types.QueryData{
		Method:    "getEnvelopeHeight",
		ProcessID: request.ProcessID,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal query")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if queryResult.Response.Code != 0 {
		sendError(router.transport, router.signer, request.context, request.id, queryResult.Response.GetInfo())
		return
	}
	apiResponse.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, apiResponse.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal height")
		return
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getEnvelopeHeight reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}

	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getBlockHeight(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	qdata := types.QueryData{
		Method: "getBlockHeight",
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal query")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if queryResult.Response.Code != 0 {
		sendError(router.transport, router.signer, request.context, request.id, "cannot fetch height")
		return
	}
	apiResponse.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, apiResponse.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot unmarshal height")
		return
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getBlockHeight reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshaling reply")
		return
	}
	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getProcessList(request routerRequest, router *Router) {
	// request.From
	// request.ListSize
	// getProcessList
}

func getEnvelopeList(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	// here we can ask to tendermint via query to get the results from the database
	qdata := types.QueryData{
		Method:    "getEnvelopeList",
		ProcessID: request.ProcessID,
		From:      request.From,
		ListSize:  request.ListSize,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal query")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if queryResult.Response.Code != 0 {
		sendError(router.transport, router.signer, request.context, request.id, queryResult.Response.GetInfo())
		return
	}
	apiResponse.Nullifiers = []string{}
	if err != nil {
		apiResponse.Nullifiers = []string{""}
	}
	if len(queryResult.Response.Value) != 0 {
		err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Nullifiers)
	} else {
		apiResponse.Nullifiers = []string{""}
	}
	if err != nil {
		log.Errorf("cannot unmarshal nullifiers: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot unmarshal nullifiers")
		return
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawAPIResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getEnvelopeList reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}
	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawAPIResponse))
}

func getResults(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	var err error
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	if len(request.ProcessID) != 64 {
		sendError(router.transport, router.signer, request.context, request.id, "processID length not valid")
		return
	}
	apiResponse.Results, err = router.Scrutinizer.VoteResult(request.ProcessID)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot get results")
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawAPIResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getEnvelopeList reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}
	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawAPIResponse))
}

func getProcListResults(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	var err error
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	apiResponse.Ok = true
	apiResponse.ProcessIDs = router.Scrutinizer.ProcessList(64)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot sign reply")
		return
	}
	rawAPIResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("error marshaling getEnvelopeList reply: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, "cannot marshal reply")
		return
	}
	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawAPIResponse))
}
