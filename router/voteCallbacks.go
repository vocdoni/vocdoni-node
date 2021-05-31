package router

import (
	"encoding/hex"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const MaxListSize = 64

func (r *Router) submitRawTx(request RouterRequest) {
	res, err := r.vocapp.SendTx(request.Payload)
	if err != nil {
		r.SendError(request, err.Error())
		return
	}
	if res == nil {
		r.SendError(request, "no reply from Vochain")
		return
	}
	if res.Code != 0 {
		r.SendError(request, string(res.Data))
		return
	}
	log.Debugf("broadcasting tx hash:%s", res.Hash)
	var response api.MetaResponse
	response.Payload = fmt.Sprintf("%x", res.Data) // return nullifier or other info
	if err = request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending raw tx: %v", err)
	}
}

func (r *Router) submitEnvelope(request RouterRequest) {
	var err error
	if request.Payload == nil {
		r.SendError(request, "payload is empty")
		return
	}
	// Prepare Vote transaction
	stx := &models.SignedTx{
		Tx:        request.Payload,
		Signature: request.Signature,
	}

	// Encode and forward the transaction to the Vochain mempool
	txBytes, err := proto.Marshal(stx)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot marshal vote transaction: (%s)", err))
		return
	}

	res, err := r.vocapp.SendTx(txBytes)
	if err != nil || res == nil {
		r.SendError(request, fmt.Sprintf("cannot broadcast transaction: (%s)", err))
		return
	}

	// Get mempool checkTx reply
	if res.Code != 0 {
		r.SendError(request, string(res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash: %s code: %d", res.Hash, res.Code)
	var response api.MetaResponse
	response.Nullifier = fmt.Sprintf("%x", res.Data)
	if err = request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error on submitEnvelope: %v", err)
	}
}

func (r *Router) getEnvelopeStatus(request RouterRequest) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		r.SendError(request, "cannot get envelope status: (malformed processId)")
		return
	}
	// check nullifier
	if len(request.Nullifier) != types.VoteNullifierSize {
		r.SendError(request, "cannot get envelope status: (malformed nullifier)")
		return
	}

	// Check envelope status and send reply
	var response api.MetaResponse
	response.Registered = types.False
	vr, err := r.Scrutinizer.GetEnvelopeReference(request.Nullifier)
	if err != nil {
		if err == scrutinizer.ErrNotFoundInDatabase {
			if err := request.Send(r.BuildReply(request, &response)); err != nil {
				log.Warnf("error sending response: %s", err)
			}
			return
		}
		r.SendError(request, fmt.Sprintf("cannot get envelope status: (%v)", err))
		return
	}
	response.Registered = types.True
	response.Height = &vr.Height
	response.BlockTimestamp = int32(vr.CreationTime.Unix())
	response.ProcessID = vr.ProcessID
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getEnvelope(request RouterRequest) {
	// check nullifier
	if len(request.Nullifier) != types.VoteNullifierSize {
		r.SendError(request, "cannot get envelope: (malformed nullifier)")
		return
	}
	env, err := r.Scrutinizer.GetEnvelope(request.Nullifier)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get envelope: (%v)", err))
		return
	}
	var response api.MetaResponse
	response.Envelope = env
	response.Registered = types.True
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getEnvelopeHeight(request RouterRequest) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize && len(request.ProcessID) != 0 {
		r.SendError(request, "cannot get envelope height: (malformed processId)")
		return
	}
	votes, err := r.Scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get envelope height: (%v)", err))
		return
	}

	var response api.MetaResponse
	response.Height = new(uint32)
	*response.Height = uint32(votes)
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getBlockHeight(request RouterRequest) {
	var response api.MetaResponse
	h := r.vocapp.Height()
	response.Height = &h
	response.BlockTimestamp = int32(r.vocapp.Timestamp())
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getProcessList(request RouterRequest) {
	var response api.MetaResponse
	max := request.ListSize
	if max > MaxListSize || max <= 0 {
		max = MaxListSize
	}
	processList, err := r.Scrutinizer.ProcessList(
		request.EntityId,
		request.From,
		max,
		request.SearchTerm,
		request.Namespace,
		request.SrcNetId,
		request.Status,
		request.WithResults)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get process list: (%s)", err))
		return
	}
	for _, p := range processList {
		response.ProcessList = append(response.ProcessList, fmt.Sprintf("%x", p))
	}
	if len(response.ProcessList) == 0 {
		response.Message = "no processes found for the query"
		if err := request.Send(r.BuildReply(request, &response)); err != nil {
			log.Warnf("error sending response: %s", err)
		}
		return
	}

	response.Size = new(int64)
	*response.Size = int64(len(response.ProcessList))
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getProcessInfo(request RouterRequest) {
	var response api.MetaResponse
	var err error
	response.Process, err = r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get process info: %v", err))
		return
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getProcessSummary(request RouterRequest) {
	var response api.MetaResponse
	if len(request.ProcessID) != types.ProcessIDsize {
		r.SendError(request, "cannot get envelope status: (malformed processId)")
		return
	}

	// Get process info
	procInfo, err := r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		r.SendError(request, err.Error())
		return
	}

	// Get total number of votes (including invalid/null)
	eh, err := r.Scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		response.Message = fmt.Sprintf("cannot get envelope height: %v", err)
		if err := request.Send(r.BuildReply(request, &response)); err != nil {
			log.Warnf("error sending response: %s", err)
		}
		return
	}
	votes := uint32(eh)

	response.ProcessSummary = &api.ProcessSummary{
		BlockCount:      procInfo.EndBlock - procInfo.StartBlock,
		EntityID:        hex.EncodeToString(procInfo.EntityID),
		EntityIndex:     procInfo.EntityIndex,
		EnvelopeHeight:  &votes,
		Metadata:        procInfo.Metadata,
		SourceNetworkID: procInfo.SourceNetworkId,
		StartBlock:      procInfo.StartBlock,
		State:           models.ProcessStatus(procInfo.Status).String(),
		EnvelopeType:    procInfo.Envelope,
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getProcessCount(request RouterRequest) {
	var response api.MetaResponse
	response.Size = new(int64)
	count := r.Scrutinizer.ProcessCount(request.EntityId)
	*response.Size = int64(count)
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getEntityCount(request RouterRequest) {
	var response api.MetaResponse
	response.Size = new(int64)
	*response.Size = int64(r.Scrutinizer.EntityCount())
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getProcessKeys(request RouterRequest) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		r.SendError(request, "cannot get envelope status: (malformed processId)")
		return
	}
	process, err := r.vocapp.State.Process(request.ProcessID, true)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get process encryption public keys: (%s)", err))
		return
	}
	var response api.MetaResponse
	var pubs, privs, coms, revs []api.Key
	for idx, pubk := range process.EncryptionPublicKeys {
		if len(pubk) > 0 {
			pubs = append(pubs, api.Key{Idx: idx, Key: pubk})
		}
	}
	for idx, privk := range process.EncryptionPrivateKeys {
		if len(privk) > 0 {
			privs = append(privs, api.Key{Idx: idx, Key: privk})
		}
	}
	for idx, comk := range process.CommitmentKeys {
		if len(comk) > 0 {
			coms = append(coms, api.Key{Idx: idx, Key: comk})
		}
	}
	for idx, revk := range process.RevealKeys {
		if len(revk) > 0 {
			revs = append(revs, api.Key{Idx: idx, Key: revk})
		}
	}
	response.EncryptionPublicKeys = pubs
	response.EncryptionPrivKeys = privs
	response.CommitmentKeys = coms
	response.RevealKeys = revs
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getResultsWeight(request RouterRequest) {
	var response api.MetaResponse
	w, err := r.Scrutinizer.GetResultsWeight(request.ProcessID)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get results weight: %v", err))
		return
	}
	response.Weight = w.String()
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getOracleResults(request RouterRequest) {
	var response api.MetaResponse
	if len(request.ProcessID) != types.ProcessIDsize {
		r.SendError(request, "cannot get oracle results: (malformed processId)")
		return
	}
	var err error
	response.Results, err = r.vocapp.State.GetProcessResults(request.ProcessID)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get oracle results: %v", err))
		return
	}
	request.Send(r.BuildReply(request, &response))
}

func (r *Router) getResults(request RouterRequest) {
	if len(request.ProcessID) != types.ProcessIDsize {
		r.SendError(request, "cannot get envelope status: (malformed processId)")
		return
	}

	var response api.MetaResponse

	// Get process info
	procInfo, err := r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		r.SendError(request, err.Error())
		return
	}

	if procInfo.Envelope.Anonymous {
		response.Type = "anonymous"
	} else {
		response.Type = "poll"
	}
	if procInfo.Envelope.EncryptedVotes {
		response.Type += " encrypted"
	} else {
		response.Type += " open"
	}
	if procInfo.Envelope.Serial {
		response.Type += " serial"
	} else {
		response.Type += " single"
	}
	response.State = models.ProcessStatus(procInfo.Status).String()

	// Get results info
	vr, err := r.Scrutinizer.GetResults(request.ProcessID)
	if err != nil && err != scrutinizer.ErrNoResultsYet {
		r.SendError(request, err.Error())
		return
	}
	if err == scrutinizer.ErrNoResultsYet {
		response.Message = scrutinizer.ErrNoResultsYet.Error()
		if err := request.Send(r.BuildReply(request, &response)); err != nil {
			log.Warnf("error sending response: %s", err)
		}
		return
	}
	if vr == nil {
		r.SendError(request, "unknown problem fetching results")
		return
	}
	response.Results = scrutinizer.GetFriendlyResults(vr.Votes)
	response.Final = &vr.Final
	h := uint32(vr.EnvelopeHeight)
	response.Height = &h
	// Get total number of votes (including invalid/null)
	eh, err := r.Scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		response.Message = fmt.Sprintf("cannot get envelope height: %v", err)
		if err := request.Send(r.BuildReply(request, &response)); err != nil {
			log.Warnf("error sending response: %s", err)
		}
		return
	}
	votes := uint32(eh)
	response.Height = &votes
	response.Weight = vr.Weight.String()
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

// known entities
func (r *Router) getEntityList(request RouterRequest) {
	var response api.MetaResponse
	if request.ListSize > MaxListSize || request.ListSize <= 0 {
		request.ListSize = MaxListSize
	}
	response.EntityIDs = r.Scrutinizer.EntityList(request.ListSize, request.From, request.SearchTerm)
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getBlockStatus(request RouterRequest) {
	var response api.MetaResponse
	h := r.vocapp.Height()
	response.Height = &h
	response.BlockTime = r.vocinfo.BlockTimes()
	response.BlockTimestamp = int32(r.vocapp.Timestamp())
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}
