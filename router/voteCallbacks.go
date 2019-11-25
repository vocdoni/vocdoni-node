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
		log.Infof("transaction hash: %s", res.Hash.String())
	}

	var apiResponse types.ResponseMessage
	apiResponse.ID = request.id
	apiResponse.Response.Request = request.id
	apiResponse.Response.Timestamp = int32(time.Now().Unix())

	if res.Code == 0 {
		apiResponse.Response.Ok = true
	} else {
		apiResponse.Response.Ok = false
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
	if err != nil {
		log.Warnf("cannot query: %s", err)
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
		if queryResult.Response.Code == 0 {
			apiResponse.Response.Registered = "true"
		} else {
			apiResponse.Response.Registered = "false"
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
	if err != nil {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
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
	if err != nil {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
	}
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
	}
	if apiResponse.Response.Height == 0 {
		apiResponse.Response.Height = -1
	}
	log.Debugf("Response height is: %d", apiResponse.Response.Height)
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
	if err != nil {
		apiResponse.Response.Ok = false
	} else {
		apiResponse.Response.Ok = true
	}
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Height)
	log.Debugf("Response height is: %d", apiResponse.Response.Height)
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
	/*
		var apiResponse types.ResponseMessage
		apiResponse.ID = request.id
		apiResponse.Response.Request = request.id
		apiResponse.Response.Timestamp = int32(time.Now().Unix())
		request.structured.From -= 1 // due to block.txs return the tx that will be included in block.Height+1

		// need to get blocks
		var i int64
		loopTimes := request.structured.From + request.structured.ListSize
		for i = 0; i < loopTimes; i++ {
			block, err := router.tmclient.Block(&i)
			if err != nil {
				// return error message
			}
			txs := block.Block.t
			for j := 0; j < block.Block.NumTxs; j++ {
				txs
			}
		}
		// here we can ask to tendermint via query to get the results from the database

		qdata := types.QueryData{
			Method:    "getEnvelopeList",
			ProcessID: request.structured.ProcessId,
			From:      request.structured.From,
			ListSize:  request.structured.ListSize,
		}
		qdataBytes, err := json.Marshal(qdata)
		if err != nil {
			log.Errorf("cannot marshal query data: (%s)", err)
		}
		queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
		if err != nil {
			apiResponse.Response.Ok = false
		} else {
			apiResponse.Response.Ok = true
		}
		err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &apiResponse.Response.Nullifiers)
		log.Debugf("Response is: %d", apiResponse.Response)
		if err != nil {
			log.Errorf("cannot unmarshal nullifiers: %s", err)
		}
		apiResponse.Signature, err = router.signer.SignJSON(apiResponse.Response)
		if err != nil {
			log.Warn(err)
		}
		rawApiResponse, err := json.Marshal(apiResponse)
		if err != nil {
			log.Errorf("Error marshaling getEnvelopeList reply: %s", err)
		}
		log.Debugf("api response: %+v", apiResponse.Response)
		router.transport.Send(buildReply(request.context, rawApiResponse))
	*/
}
