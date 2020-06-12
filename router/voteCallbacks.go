package router

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

const MaxListSize = 256

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
	response.Payload = string(res.Data) // return nullifier
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) submitEnvelope(request routerRequest) {
	voteTxArgs := new(types.VoteTx)
	if request.Payload == nil {
		r.sendError(request, "payload is empty")
		return
	}
	voteTxArgs.ProcessID = request.Payload.ProcessID
	voteTxArgs.Nonce = request.Payload.Nonce
	voteTxArgs.Nullifier = request.Payload.Nullifier
	voteTxArgs.VotePackage = request.Payload.VotePackage
	voteTxArgs.Proof = request.Payload.Proof
	voteTxArgs.Type = "vote"
	voteTxArgs.EncryptionKeyIndexes = request.Payload.EncryptionKeyIndexes
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
	// check pid
	request.ProcessID = util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(request.ProcessID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope status: (malformed processId)")
		return
	}
	// check nullifier
	request.Nullifier = util.TrimHex(request.Nullifier)
	if !util.IsHexEncodedStringWithLength(request.Nullifier, types.VoteNullifierSize) {
		r.sendError(request, "cannot get envelope status: (malformed nullifier)")
		return
	}

	// Check envelope status and send reply
	var response types.ResponseMessage
	response.Registered = types.False

	e, err := r.vocapp.State.Envelope(fmt.Sprintf("%s_%s", request.ProcessID, request.Nullifier), true)
	// Warning, error is ignored. We should find a better way to check the envelope status
	if err == nil && e != nil {
		response.Registered = types.True
		response.Nullifier = e.Nullifier
		response.Height = &e.Height
		block := r.vocapp.Node.BlockStore().LoadBlock(e.Height)
		if block == nil {
			r.sendError(request, "failed getting envelope block timestamp")
			return
		}
		response.BlockTimestamp = int32(block.Time.Unix())
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelope(request routerRequest) {
	// check pid
	sanitizedPID := util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(sanitizedPID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope: (malformed processId)")
		return
	}
	// check nullifier
	sanitizedNullifier := util.TrimHex(request.Nullifier)
	if !util.IsHexEncodedStringWithLength(sanitizedNullifier, types.VoteNullifierSize) {
		r.sendError(request, "cannot get envelope: (malformed nullifier)")
		return
	}
	envelope, err := r.vocapp.State.Envelope(fmt.Sprintf("%s_%s", sanitizedPID, sanitizedNullifier), true)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get envelope: (%s)", err))
		return
	}
	var response types.ResponseMessage
	response.Registered = types.True
	response.Payload = envelope.VotePackage
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelopeHeight(request routerRequest) {
	// check pid
	sanitizedPID := util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(sanitizedPID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope height: (malformed processId)")
		return
	}
	votes := r.vocapp.State.CountVotes(sanitizedPID, true)
	var response types.ResponseMessage
	response.Height = new(int64)
	*response.Height = votes
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getBlockHeight(request routerRequest) {
	var response types.ResponseMessage
	response.Height = &r.vocapp.State.Header(true).Height
	response.BlockTimestamp = int32(r.vocapp.State.Header(true).Time.Unix())
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getProcessList(request routerRequest) {
	// check eid
	request.EntityId = util.TrimHex(request.EntityId)
	if !util.IsHexEncodedStringWithLength(request.EntityId, types.EntityIDsize) &&
		!util.IsHexEncodedStringWithLength(request.EntityId, types.EntityIDsizeV2) {
		r.sendError(request, "cannot get process list: (malformed entityId)")
		return
	}
	storageKey := []byte(types.ScrutinizerEntityPrefix + request.EntityId)
	var response types.ResponseMessage
	exists, err := r.Scrutinizer.Storage.Has(storageKey)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get entity (%s)", err))
		return
	}
	if !exists {
		response.Message = "entity does not exist or has not yet created a process"
		r.transport.Send(r.buildReply(request, response))
		return
	}
	processList, err := r.Scrutinizer.Storage.Get(storageKey)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get entity process list: (%s)", err))
		return
	}
	for _, process := range bytes.Split(processList, []byte(types.ScrutinizerEntityProcessSeparator)) {
		if len(process) > 0 {
			response.ProcessList = append(response.ProcessList, string(process))
		}
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getProcessKeys(request routerRequest) {
	// check pid
	sanitizedPID := util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(sanitizedPID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope height: (malformed processId)")
		return
	}
	process, err := r.vocapp.State.Process(sanitizedPID, true)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get process encryption public keys: (%s)", err))
		return
	}
	var response types.ResponseMessage
	var pubs, privs, coms, revs []types.Key
	for idx, pubk := range process.EncryptionPublicKeys {
		if len(pubk) > 0 {
			pubs = append(pubs, types.Key{Idx: idx, Key: pubk})
		}
	}
	for idx, privk := range process.EncryptionPrivateKeys {
		if len(privk) > 0 {
			privs = append(privs, types.Key{Idx: idx, Key: privk})
		}
	}
	for idx, comk := range process.CommitmentKeys {
		if len(comk) > 0 {
			coms = append(coms, types.Key{Idx: idx, Key: comk})
		}
	}
	for idx, revk := range process.RevealKeys {
		if len(revk) > 0 {
			revs = append(revs, types.Key{Idx: idx, Key: revk})
		}
	}
	response.EncryptionPublicKeys = pubs
	response.EncryptionPrivKeys = privs
	response.CommitmentKeys = coms
	response.RevealKeys = revs
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getEnvelopeList(request routerRequest) {
	if !util.IsHexEncodedStringWithLength(request.ProcessID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope list: (malformed processId)")
		return
	}
	if request.ListSize > MaxListSize {
		r.sendError(request, fmt.Sprintf("listSize overflow, maximum is %d", MaxListSize))
		return
	}
	if request.ListSize == 0 {
		request.ListSize = 64
	}
	n := r.vocapp.State.EnvelopeList(request.ProcessID, request.From, request.ListSize, true)
	var response types.ResponseMessage
	if len(n) == 0 {
		response.Nullifiers = &[]string{}
	} else {
		response.Nullifiers = &n
	}
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) getResults(request routerRequest) {
	var err error
	sanitizedPID := util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(sanitizedPID, types.ProcessIDsize) {
		r.sendError(request, "cannot get results: (malformed processId)")
		return
	}
	request.ProcessID = sanitizedPID
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
