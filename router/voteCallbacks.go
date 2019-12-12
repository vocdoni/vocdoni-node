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
	}

	res, err := router.tmclient.BroadcastTxSync(voteTxBytes)
	if err != nil {
		log.Warnf("cannot broadcast tx: %s", err)
	} else {
		log.Infof("transaction hash: %s, transaction code: %d", res.Hash, res.Code)
	}

	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())

	apiResponse.Ok = types.Bool(res.Code == 0)

	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling submitEnvelope reply: %s", err)
	}
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelopeStatus(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method:    "getEnvelopeStatus",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: %s", err)
		apiResponse.Ok = types.False
		apiResponse.Registered = types.False
	} else {
		apiResponse.Ok = types.Bool(queryResult.Response.Code == 0)
	}

	log.Debugf("Response is: %d", apiResponse.MetaResponse)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelopeStatus reply: %s", err)
	}

	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelope(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method:    "getEnvelope",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Ok = types.Bool(err == nil)

	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Payload)
	if err != nil {
		log.Errorf("cannot unmarshal vote package: %s", err)
	}
	log.Debugf("Response is: %d", apiResponse.MetaResponse)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelope reply: %s", err)
	}

	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelopeHeight(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method:    "getEnvelopeHeight",
		ProcessID: request.ProcessID,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Ok = types.Bool(err == nil)

	apiResponse.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, apiResponse.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
	}
	log.Debugf("Response height is: %d", *apiResponse.Height)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelopeHeight reply: %s", err)
	}

	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getBlockHeight(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Request = request.id
	apiResponse.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method: "getBlockHeight",
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Ok = types.Bool(err == nil)

	apiResponse.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, apiResponse.Height)
	log.Debugf("Response height is: %d", *apiResponse.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getBlockHeight reply: %s", err)
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
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Nullifiers = new([]string)
	if err != nil {
		*apiResponse.Nullifiers = []string{""}
	}
	if len(queryResult.Response.Value) != 0 {
		err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Nullifiers)
	} else {
		*apiResponse.Nullifiers = []string{""}
	}
	log.Debugf("Response is: %d", apiResponse.MetaResponse)
	if err != nil {
		log.Errorf("cannot unmarshal nullifiers: %s", err)
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawAPIResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelopeList reply: %s", err)
	}
	log.Debugf("api response: %+v", apiResponse.MetaResponse)
	router.transport.Send(buildReply(request.context, rawAPIResponse))
}
