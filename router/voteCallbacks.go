package router

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

const MaxListSize = 256
const MaxListIterations = int64(64)

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
		log.Warnf("cannot broadcast tx (res.Code=%d): (%s)", res.Code, res.Data)
		r.sendError(request, fmt.Sprintf("%x", res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash:%s code:%d", res.Hash, res.Code)
	var response types.MetaResponse
	response.Payload = fmt.Sprintf("%x", res.Data) // return nullifier or other info
	request.Send(r.buildReply(request, &response))
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

	res, err := r.vocapp.SendTX(voteTxBytes)
	if err != nil || res == nil {
		log.Warnf("cannot broadcast tx: (%s)", err)
		r.sendError(request, fmt.Sprintf("cannot broadcast tx: %s", err))
		return
	}
	if res.Code != 0 {
		log.Warnf("cannot broadcast tx (res.Code=%d): (%s)", res.Code, res.Data)
		r.sendError(request, string(res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash:%s code:%d", res.Hash, res.Code)
	var response types.MetaResponse
	response.Nullifier = fmt.Sprintf("%x", res.Data)
	request.Send(r.buildReply(request, &response))
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
	var response types.MetaResponse
	response.Registered = types.False

	pid, err := hex.DecodeString(request.ProcessID)
	if err != nil {
		r.sendError(request, "cannot decode processID")
		return
	}
	nullifier, err := hex.DecodeString(request.Nullifier)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot decode nullifier: (%s)", err))
		return
	}
	e, err := r.vocapp.State.Envelope(pid, nullifier, true)
	// Warning, error is ignored. We should find a better way to check the envelope status
	if err == nil && e != nil {
		response.Registered = types.True
		response.Nullifier = fmt.Sprintf("%x", e.Nullifier)
		response.Height = &e.Height
		block := r.vocapp.Node.BlockStore().LoadBlock(e.Height)
		if block == nil {
			r.sendError(request, "failed getting envelope block timestamp")
			return
		}
		response.BlockTimestamp = int32(block.Time.Unix())
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelope(request routerRequest) {
	// check pid
	if !util.IsHexEncodedStringWithLength(request.ProcessID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope: (malformed processId)")
		return
	}
	// check nullifier
	request.Nullifier = util.TrimHex(request.Nullifier)
	if !util.IsHexEncodedStringWithLength(request.Nullifier, types.VoteNullifierSize) {
		r.sendError(request, "cannot get envelope: (malformed nullifier)")
		return
	}
	pid, err := hex.DecodeString(util.TrimHex(request.ProcessID))
	if err != nil {
		r.sendError(request, "cannot decode processID")
		return
	}
	nullifier, err := hex.DecodeString(util.TrimHex(request.Nullifier))
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot decode nullifier: (%s)", err))
		return
	}
	envelope, err := r.vocapp.State.Envelope(pid, nullifier, true)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get envelope: (%s)", err))
		return
	}
	var response types.MetaResponse
	response.Registered = types.True
	response.Payload = envelope.VotePackage
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelopeHeight(request routerRequest) {
	// check pid
	request.ProcessID = util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(request.ProcessID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope height: (malformed processId)")
		return
	}
	pid, err := hex.DecodeString(request.ProcessID)
	if err != nil {
		r.sendError(request, "cannot decode processID")
		return
	}
	votes := r.vocapp.State.CountVotes(pid, true)
	var response types.MetaResponse
	response.Height = new(int64)
	*response.Height = votes
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockHeight(request routerRequest) {
	var response types.MetaResponse
	response.Height = &r.vocapp.State.Header(true).Height
	response.BlockTimestamp = int32(r.vocapp.State.Header(true).Time.Unix())
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessList(request routerRequest) {
	// check/sanitize eid and fromId
	request.EntityId = util.TrimHex(request.EntityId)
	if !util.IsHexEncodedStringWithLength(request.EntityId, types.EntityIDsize) &&
		!util.IsHexEncodedStringWithLength(request.EntityId, types.EntityIDsizeV2) {
		r.sendError(request, "cannot get process list: (malformed entityId)")
		return
	}
	if len(request.FromID) > 0 {
		request.FromID = util.TrimHex(request.FromID)
		if !util.IsHexEncodedStringWithLength(request.FromID, types.ProcessIDsize) {
			r.sendError(request, "cannot get process list: (malformed fromId)")
			return
		}
	}
	eid, err := hex.DecodeString(request.EntityId)
	if err != nil {
		r.sendError(request, "cannot decode entityID")
		return
	}
	fromID := []byte{}
	if request.FromID != "" {
		fromID, err = hex.DecodeString(request.FromID)
		if err != nil {
			r.sendError(request, "cannot decode fromID")
			return
		}
	}

	var response types.MetaResponse
	max := request.ListSize
	if max > MaxListIterations || max <= 0 {
		max = MaxListIterations
	}
	response.ProcessList, err = r.Scrutinizer.ProcessList(eid, fromID, max)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get entity process list: (%s)", err))
		return
	}
	if len(response.ProcessList) == 0 {
		response.Message = "entity does not exist or has not yet created a process"
		request.Send(r.buildReply(request, &response))
		return
	}

	response.Size = new(int64)
	*response.Size = int64(len(response.ProcessList))
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessCount(request routerRequest) {
	var response types.MetaResponse
	response.Size = new(int64)
	count := r.vocapp.State.CountProcesses(true)
	*response.Size = count
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getScrutinizerEntityCount(request routerRequest) {
	var response types.MetaResponse
	response.Size = new(int64)
	*response.Size = r.Scrutinizer.EntityCount()
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessKeys(request routerRequest) {
	// check pid
	request.ProcessID = util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(request.ProcessID, types.ProcessIDsize) {
		r.sendError(request, "cannot get envelope height: (malformed processId)")
		return
	}
	pid, err := hex.DecodeString(request.ProcessID)
	if err != nil {
		r.sendError(request, "cannot decode processID")
		return
	}
	process, err := r.vocapp.State.Process(pid, true)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get process encryption public keys: (%s)", err))
		return
	}
	var response types.MetaResponse
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
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelopeList(request routerRequest) {
	request.ProcessID = util.TrimHex(request.ProcessID)
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
	pid, err := hex.DecodeString(request.ProcessID)
	if err != nil {
		r.sendError(request, "cannot decode processID")
		return
	}
	nullifiers := r.vocapp.State.EnvelopeList(pid, request.From, request.ListSize, true)
	var response types.MetaResponse
	strnull := []string{}
	for _, n := range nullifiers {
		strnull = append(strnull, fmt.Sprintf("%x", n))
	}
	response.Nullifiers = &strnull
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getResults(request routerRequest) {
	var err error
	request.ProcessID = util.TrimHex(request.ProcessID)
	if !util.IsHexEncodedStringWithLength(request.ProcessID, types.ProcessIDsize) {
		r.sendError(request, "cannot get results: (malformed processId)")
		return
	}

	var response types.MetaResponse

	// Get process info
	pid, err := hex.DecodeString(request.ProcessID)
	if err != nil {
		r.sendError(request, "cannot decode processID")
		return
	}
	procInfo, err := r.Scrutinizer.ProcessInfo(pid)
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

	// Get results info
	vr, err := r.Scrutinizer.VoteResult(pid)
	if err != nil && err != scrutinizer.ErrNoResultsYet {
		log.Warn(err)
		r.sendError(request, err.Error())
		return
	}
	if err == scrutinizer.ErrNoResultsYet {
		response.Message = scrutinizer.ErrNoResultsYet.Error()
	}
	response.Results = vr

	// Get number of votes
	votes := r.vocapp.State.CountVotes(pid, true)
	response.Height = new(int64)
	*response.Height = votes

	request.Send(r.buildReply(request, &response))
}

// finished processes
func (r *Router) getProcListResults(request routerRequest) {
	var response types.MetaResponse
	if request.ListSize > MaxListIterations || request.ListSize <= 0 {
		request.ListSize = MaxListIterations
	}
	var err error
	response.ProcessIDs, err = r.Scrutinizer.ProcessListWithResults(request.ListSize, request.FromID)
	if err != nil {
		r.sendError(request, "cannot decode fromID")
		return
	}
	request.Send(r.buildReply(request, &response))
}

// live processes
func (r *Router) getProcListLiveResults(request routerRequest) {
	var response types.MetaResponse
	if request.ListSize > MaxListIterations || request.ListSize <= 0 {
		request.ListSize = MaxListIterations
	}
	var err error
	response.ProcessIDs, err = r.Scrutinizer.ProcessListWithLiveResults(request.ListSize, request.FromID)
	if err != nil {
		r.sendError(request, err.Error())
		return
	}

	request.Send(r.buildReply(request, &response))
}

// known entities
func (r *Router) getScrutinizerEntities(request routerRequest) {
	var response types.MetaResponse
	if request.ListSize > MaxListIterations || request.ListSize <= 0 {
		request.ListSize = MaxListIterations
	}
	var err error
	response.EntityIDs, err = r.Scrutinizer.EntityList(request.ListSize, request.FromID)
	if err != nil {
		r.sendError(request, err.Error())
		return
	}

	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockStatus(request routerRequest) {
	var response types.MetaResponse
	response.BlockTime = r.vocinfo.BlockTimes()
	response.Height = &r.vocapp.State.Header(true).Height
	response.BlockTimestamp = int32(r.vocapp.State.Header(true).Time.Unix())
	request.Send(r.buildReply(request, &response))
}
