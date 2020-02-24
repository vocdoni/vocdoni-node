package router

import (
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
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
		router.sendError(request, "cannot marshal voteTx args")
		return
	}

	res, err := router.tmclient.BroadcastTxSync(voteTxBytes)
	if err != nil || res.Code != 0 {
		log.Warnf("cannot broadcast tx (res.Code=%d): %s || %s", res.Code, err, string(res.Data))
		router.sendError(request, string(res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash:%s code:%d", res.Hash, res.Code)
	var response types.ResponseMessage
	router.transport.Send(router.buildReply(request, response))
}

func getEnvelopeStatus(request routerRequest, router *Router) {
	qdata := types.QueryData{
		Method:    "getEnvelopeStatus",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		router.sendError(request, "cannot marshal query data")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: %s", err)
		router.sendError(request, "cannot query")
		return
	}
	var response types.ResponseMessage
	response.Registered = types.True
	if queryResult.Response.Code != 0 {
		response.Registered = types.False
	}
	router.transport.Send(router.buildReply(request, response))
}

func getEnvelope(request routerRequest, router *Router) {
	qdata := types.QueryData{
		Method:    "getEnvelope",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		router.sendError(request, "cannot marshal query data")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: %s", err)
		router.sendError(request, "cannot query")
		return
	}
	if queryResult.Response.Code != 0 {
		router.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	if err := router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &response.Payload); err != nil {
		log.Errorf("cannot unmarshal vote package: %s", err)
		router.sendError(request, "cannot unmarshal vote package")
		return
	}
	router.transport.Send(router.buildReply(request, response))
}

func getEnvelopeHeight(request routerRequest, router *Router) {
	qdata := types.QueryData{
		Method:    "getEnvelopeHeight",
		ProcessID: request.ProcessID,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: %s", err)
		router.sendError(request, "cannot marshal query")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil || queryResult.Response.Code != 0 {
		router.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	response.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
		router.sendError(request, "cannot marshal height")
		return
	}
	router.transport.Send(router.buildReply(request, response))
}

func getBlockHeight(request routerRequest, router *Router) {
	qdata := types.QueryData{
		Method: "getBlockHeight",
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		router.sendError(request, "cannot marshal query")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if err != nil || queryResult.Response.Code != 0 {
		router.sendError(request, "cannot fetch height")
		return
	}
	var response types.ResponseMessage
	response.Height = new(int64)
	err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: %s", err)
		router.sendError(request, "cannot unmarshal height")
		return
	}
	router.transport.Send(router.buildReply(request, response))
}

func getProcessList(request routerRequest, router *Router) {
	queryResult, err := router.tmclient.TxSearch(fmt.Sprintf("processCreated.entityId='%s'", util.TrimHex(request.EntityId)), false, 1, 30)
	if err != nil {
		log.Errorf("cannot query: %s", err)
		router.sendError(request, err.Error())
		return
	}
	var processList []string
	for _, res := range queryResult.Txs {
		for _, evt := range res.TxResult.Events {
			processList = append(processList, string(evt.Attributes[1].Value))
		}
	}
	var response types.ResponseMessage
	if len(processList) == 0 {
		response.ProcessList = []string{""}
	} else {
		response.ProcessList = processList
	}
	router.transport.Send(router.buildReply(request, response))
}

func getEnvelopeList(request routerRequest, router *Router) {
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
		router.sendError(request, "cannot marshal query")
		return
	}
	queryResult, err := router.tmclient.ABCIQuery("", qdataBytes)
	if queryResult.Response.Code != 0 {
		router.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	response.Nullifiers = []string{}
	if err != nil {
		response.Nullifiers = []string{""}
	}
	if len(queryResult.Response.Value) != 0 {
		err = router.codec.UnmarshalBinaryBare(queryResult.Response.Value, &response.Nullifiers)
	} else {
		response.Nullifiers = []string{""}
	}
	if err != nil {
		log.Errorf("cannot unmarshal nullifiers: %s", err)
		router.sendError(request, "cannot unmarshal nullifiers")
		return
	}
	router.transport.Send(router.buildReply(request, response))
}

func getResults(request routerRequest, router *Router) {
	var err error
	request.ProcessID = util.TrimHex(request.ProcessID)
	if len(request.ProcessID) != 64 {
		router.sendError(request, "processID length not valid")
		return
	}

	var response types.ResponseMessage
	response.Results, err = router.Scrutinizer.VoteResult(request.ProcessID)
	if err != nil {
		log.Warn(err)
		router.sendError(request, "cannot get results")
		return
	}

	procInfo, err := router.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		router.sendError(request, "cannot get process info")
		return
	}
	response.Type = procInfo.Type
	if procInfo.Canceled {
		response.State = "canceled"
	} else {
		response.State = "active"
	}
	router.transport.Send(router.buildReply(request, response))
}

func getProcListResults(request routerRequest, router *Router) {
	var response types.ResponseMessage
	response.ProcessIDs = router.Scrutinizer.ProcessList(64, util.TrimHex(request.FromID))
	router.transport.Send(router.buildReply(request, response))
}
