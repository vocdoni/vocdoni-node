package router

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

func (r *Router) submitRawTx(request routerRequest) {
	tx, err := base64.StdEncoding.DecodeString(request.RawTx)
	if err != nil {
		log.Warnf("error decoding base64 raw tx: (%s)", err)
		r.sendError(request, err.Error())
		return
	}
	res, err := r.vocapp.SendTX(tx)
	if err != nil {
		r.sendError(request, err.Error())
		return
	}
	if res == nil {
		r.sendError(request, "no reply from Vochain")
		return
	}
	if res.Code != 0 {
		log.Warnf("cannot broadcast tx (res.Code=%d): (%s)", res.Code, string(res.Data))
		r.sendError(request, string(res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash:%s code:%d", res.Hash, res.Code)
	var response types.ResponseMessage
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) submitEnvelope(request routerRequest) {
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
		log.Errorf("error marshaling voteTx args: (%s)", err)
		r.sendError(request, "cannot marshal voteTx args")
		return
	}

	//res, err := r.tmclient.BroadcastTxSync(voteTxBytes)
	res, err := r.vocapp.SendTX(voteTxBytes)
	if err != nil || res == nil {
		log.Warnf("cannot broadcast tx: (%s)", err)
		r.sendError(request, "cannot broadcast TX")
		return
	}
	if res.Code != 0 {
		log.Warnf("cannot broadcast tx (res.Code=%d): (%s)", res.Code, string(res.Data))
		r.sendError(request, string(res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash:%s code:%d", res.Hash, res.Code)
	var response types.ResponseMessage
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelopeStatus(request routerRequest) {
	qdata := types.QueryData{
		Method:    "getEnvelopeStatus",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		r.sendError(request, "cannot marshal query data")
		return
	}
	queryResult, err := r.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: (%s)", err)
		r.sendError(request, "cannot query")
		return
	}
	var response types.ResponseMessage
	response.Registered = types.True
	if queryResult.Response.Code != 0 {
		response.Registered = types.False
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelope(request routerRequest) {
	qdata := types.QueryData{
		Method:    "getEnvelope",
		ProcessID: request.ProcessID,
		Nullifier: request.Nullifier,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		r.sendError(request, "cannot marshal query data")
		return
	}
	queryResult, err := r.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: (%s)", err)
		r.sendError(request, "cannot query")
		return
	}
	if queryResult.Response.Code != 0 {
		r.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	if err := r.codec.UnmarshalBinaryBare(queryResult.Response.Value, &response.Payload); err != nil {
		log.Errorf("cannot unmarshal vote package: (%s)", err)
		r.sendError(request, "cannot unmarshal vote package")
		return
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelopeHeight(request routerRequest) {
	qdata := types.QueryData{
		Method:    "getEnvelopeHeight",
		ProcessID: request.ProcessID,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		r.sendError(request, "cannot marshal query")
		return
	}
	queryResult, err := r.tmclient.ABCIQuery("", qdataBytes)
	if err != nil || queryResult.Response.Code != 0 {
		r.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	response.Height = new(int64)
	err = r.codec.UnmarshalBinaryBare(queryResult.Response.Value, response.Height)
	if err != nil {
		log.Errorf("cannot unmarshal height: (%s)", err)
		r.sendError(request, "cannot marshal height")
		return
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getBlockHeight(request routerRequest) {
	var response types.ResponseMessage
	response.Height = &r.vocapp.State.Header().Height
	response.BlockTimestamp = int32(r.vocapp.State.Header().Time.Unix())
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getProcessList(request routerRequest) {
	queryResult, err := r.tmclient.TxSearch(fmt.Sprintf("processCreated.entityId='%s'", util.TrimHex(request.EntityId)), false, 1, 30, "asc")
	if err != nil {
		log.Errorf("cannot query: (%s)", err)
		r.sendError(request, "cannot query")
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
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getProcessKeys(request routerRequest) {
	qdata := types.QueryData{
		Method:    "getProcessKeys",
		ProcessID: request.ProcessID,
	}
	qdataBytes, err := json.Marshal(qdata)
	if err != nil {
		log.Errorf("cannot marshal query data: (%s)", err)
		r.sendError(request, "cannot marshal query data")
		return
	}
	queryResult, err := r.tmclient.ABCIQuery("", qdataBytes)
	if err != nil {
		log.Warnf("cannot query: (%s)", err)
		r.sendError(request, "cannot query")
		return
	}
	if queryResult.Response.Code != 0 {
		r.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	if err := r.codec.UnmarshalBinaryBare(queryResult.Response.Value, &response.ProcessKeys); err != nil {
		log.Errorf("cannot unmarshal process keys: (%s)", err)
		r.sendError(request, "cannot unmarshal process keys")
		return
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelopeList(request routerRequest) {
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
		r.sendError(request, "cannot marshal query")
		return
	}
	queryResult, err := r.tmclient.ABCIQuery("", qdataBytes)
	if queryResult.Response.Code != 0 {
		r.sendError(request, queryResult.Response.GetInfo())
		return
	}
	var response types.ResponseMessage
	response.Nullifiers = []string{}
	if err != nil {
		response.Nullifiers = []string{""}
	}
	if len(queryResult.Response.Value) != 0 {
		err = r.codec.UnmarshalBinaryBare(queryResult.Response.Value, &response.Nullifiers)
	} else {
		response.Nullifiers = []string{""}
	}
	if err != nil {
		log.Errorf("cannot unmarshal nullifiers: (%s)", err)
		r.sendError(request, "cannot unmarshal nullifiers")
		return
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getResults(request routerRequest) {
	request.ProcessID = util.TrimHex(request.ProcessID)
	if len(request.ProcessID) != 64 {
		r.sendError(request, "processID length not valid")
		return
	}

	vr, err := r.Scrutinizer.VoteResult(request.ProcessID)
	if err != nil {
		log.Warn(err)
		r.sendError(request, err.Error())
		return
	}
	var response types.ResponseMessage
	response.Results = vr

	procInfo, err := r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		r.sendError(request, err.Error())
		return
	}
	response.Type = procInfo.Type
	if procInfo.Canceled {
		response.State = "canceled"
	} else {
		response.State = "active"
	}
	r.transport.Send(r.buildReply(request, response))
}

// finished processes
func (r *Router) getProcListResults(request routerRequest) {
	var response types.ResponseMessage
	response.ProcessIDs = r.Scrutinizer.List(64, util.TrimHex(request.FromID), types.ScrutinizerResultsPrefix)
	r.transport.Send(r.buildReply(request, response))
}

// live processes
func (r *Router) getProcListLiveResults(request routerRequest) {
	var response types.ResponseMessage
	response.ProcessIDs = r.Scrutinizer.List(64, util.TrimHex(request.FromID), types.ScrutinizerLiveProcessPrefix)
	r.transport.Send(r.buildReply(request, response))
}

// known entities
func (r *Router) getScrutinizerEntities(request routerRequest) {
	var response types.ResponseMessage
	response.EntityIDs = r.Scrutinizer.List(64, util.TrimHex(request.FromID), types.ScrutinizerEntityPrefix)
	r.transport.Send(r.buildReply(request, response))
}
