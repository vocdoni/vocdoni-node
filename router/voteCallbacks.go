package router

import (
	"encoding/json"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func submitEnvelope(request routerRequest, router *Router) {
	voteTxArgs := new(types.VoteTx)
	voteTxArgs.ProcessID = request.structured.Payload.ProcessID
	voteTxArgs.Nonce = request.structured.Payload.Nonce
	voteTxArgs.Nullifier = request.structured.Payload.Nullifier
	voteTxArgs.VotePackage = request.structured.Payload.VotePackage
	voteTxArgs.Proof = request.structured.Payload.Proof
	voteTxArgs.Type = "vote"
	voteTxArgs.Signature = request.structured.Payload.Signature

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
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())

	ok := new(bool)
	if res.Code == 0 {
		*ok = true
		apiResponse.Response.Ok = ok
	} else {
		*ok = false
		apiResponse.Response.Ok = ok
	}

	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
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
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method:    "getEnvelopeStatus",
		ProcessID: request.structured.ProcessID,
		Nullifier: request.structured.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	ok := new(bool)
	if err != nil {
		log.Warnf("cannot query: %s", err)
		*ok = false
		apiResponse.Response.Ok = ok
		apiResponse.Response.Registered = ok
	} else {
		*ok = true
		apiResponse.Response.Ok = ok
		registered := new(bool)
		if queryResult.Response.Code == 0 {
			*registered = true
			apiResponse.Response.Registered = registered
		} else {
			*registered = false
			apiResponse.Response.Registered = registered
		}
	}

	log.Debugf("Response is: %d", apiResponse.Response)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelopeStatus reply: %s", err)
	}

	log.Debugf("api response: %+v", apiResponse.Response)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelope(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method:    "getEnvelope",
		ProcessID: request.structured.ProcessID,
		Nullifier: request.structured.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	ok := new(bool)
	if err != nil {
		*ok = false
		apiResponse.Response.Ok = ok
	} else {
		*ok = true
		apiResponse.Response.Ok = ok
	}
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Payload)
	if err != nil {
		log.Errorf("cannot unmarshal vote package: %s", err)
	}
	log.Debugf("Response is: %d", apiResponse.Response)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err)
	}
	rawApiResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelope reply: %s", err)
	}

	log.Debugf("api response: %+v", apiResponse.Response)
	router.transport.Send(buildReply(request.context, rawApiResponse))
}

func getEnvelopeHeight(request routerRequest, router *Router) {
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())
	qdata := types.QueryData{
		Method:    "getEnvelopeHeight",
		ProcessID: request.structured.ProcessID,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Response.Ok = new(bool)
	if err != nil {
		*apiResponse.Response.Ok = false
	} else {
		*apiResponse.Response.Ok = true
	}
	apiResponse.Response.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, apiResponse.Response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
	}
	log.Debugf("Response height is: %d", *apiResponse.Response.Height)
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err)
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
	qdata := types.QueryData{
		Method: "getBlockHeight",
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Response.Ok = new(bool)
	if err != nil {
		*apiResponse.Response.Ok = false
	} else {
		*apiResponse.Response.Ok = true
	}
	apiResponse.Response.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, apiResponse.Response.Height)
	log.Debugf("Response height is: %d", *apiResponse.Response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err)
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
	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())

	// here we can ask to tendermint via query to get the results from the database
	qdata := types.QueryData{
		Method:    "getEnvelopeList",
		ProcessID: request.structured.ProcessID,
		From:      request.structured.From,
		ListSize:  request.structured.ListSize,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	apiResponse.Response.Nullifiers = new([]string)
	if err != nil {
		*apiResponse.Response.Nullifiers = []string{""}
	}
	if len(queryResult.Response.Value) != 0 {
		err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Nullifiers)
	} else {
		*apiResponse.Response.Nullifiers = []string{""}
	}
	log.Debugf("Response is: %d", apiResponse.Response)
	if err != nil {
		log.Errorf("cannot unmarshal nullifiers: %s", err)
	}
	apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
	if err != nil {
		log.Warn(err)
	}
	rawAPIResponse, err := json.Marshal(apiResponse)
	if err != nil {
		log.Errorf("Error marshaling getEnvelopeList reply: %s", err)
	}
	log.Debugf("api response: %+v", apiResponse.Response)
	router.transport.Send(buildReply(request.context, rawAPIResponse))
}
