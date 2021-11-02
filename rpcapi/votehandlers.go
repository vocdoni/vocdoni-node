package rpcapi

import (
	"encoding/hex"
	"errors"
	"fmt"

	api "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func (r *RPCAPI) submitRawTx(request *api.APIrequest) (*api.APIresponse, error) {
	res, err := r.vocapp.SendTx(request.Payload)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("no reply from vochain")
	}
	if res.Code != 0 {
		return nil, fmt.Errorf("%s", res.Data)
	}
	log.Debugf("broadcasting tx hash:%s", res.Hash)
	// return nullifier or other info
	return &api.APIresponse{Payload: fmt.Sprintf("%x", res.Data)}, nil
}

func (a *RPCAPI) submitEnvelope(request *api.APIrequest) (*api.APIresponse, error) {
	var err error
	if request.Payload == nil {
		return nil, fmt.Errorf("payload is empty")
	}

	// Prepare Vote transaction
	stx := &models.SignedTx{
		Tx:        request.Payload,
		Signature: request.Signature,
	}

	// Encode and forward the transaction to the Vochain mempool
	txBytes, err := proto.Marshal(stx)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal vote transaction: (%s)", err)
	}

	res, err := a.vocapp.SendTx(txBytes)
	if err != nil || res == nil {
		return nil, fmt.Errorf("cannot broadcast transaction: (%s)", err)
	}

	// Get mempool checkTx reply
	if res.Code != 0 {
		return nil, fmt.Errorf("%v", res.Data)
	}
	log.Infof("broadcasting vochain tx hash: %s code: %d", res.Hash, res.Code)
	return &api.APIresponse{Nullifier: fmt.Sprintf("%x", res.Data)}, nil
}

func (r *RPCAPI) getEnvelopeStatus(request *api.APIrequest) (*api.APIresponse, error) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		return nil, fmt.Errorf("cannot get envelope status: (malformed processId)")

	}
	// check nullifier
	if len(request.Nullifier) != types.VoteNullifierSize {
		return nil, fmt.Errorf("cannot get envelope status: (malformed nullifier)")
	}

	// Check envelope status and send reply
	var response api.APIresponse
	response.Registered = types.False
	vr, err := r.scrutinizer.GetEnvelopeReference(request.Nullifier)
	if err != nil {
		if errors.Is(err, scrutinizer.ErrNotFoundInDatabase) {
			return &response, nil
		}
		return nil, fmt.Errorf("cannot get envelope status: (%v)", err)
	}
	response.Registered = types.True
	response.Height = &vr.Height
	response.BlockTimestamp = int32(vr.CreationTime.Unix())
	response.ProcessID = vr.ProcessID
	return &response, nil
}

func (r *RPCAPI) getEnvelope(request *api.APIrequest) (*api.APIresponse, error) {
	// check nullifier
	if len(request.Nullifier) != types.VoteNullifierSize {
		return nil, fmt.Errorf("cannot get envelope: (malformed nullifier)")
	}
	env, err := r.scrutinizer.GetEnvelope(request.Nullifier)
	if err != nil {
		return nil, fmt.Errorf("cannot get envelope: (%v)", err)
	}
	var response api.APIresponse
	response.Envelope = env
	response.Registered = types.True
	return &response, nil
}

func (r *RPCAPI) getEnvelopeHeight(request *api.APIrequest) (*api.APIresponse, error) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize && len(request.ProcessID) != 0 {
		return nil, fmt.Errorf("cannot get envelope height: (malformed processId)")
	}
	votes, err := r.scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		return nil, fmt.Errorf("cannot get envelope height: (%v)", err)
	}
	var response api.APIresponse
	response.Height = new(uint32)
	*response.Height = uint32(votes)
	return &response, nil
}

func (r *RPCAPI) getBlockHeight(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	h := r.vocapp.Height()
	response.Height = &h
	response.BlockTimestamp = int32(r.vocapp.Timestamp())
	return &response, nil
}

func (r *RPCAPI) getProcessList(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	max := request.ListSize
	if max > MaxListSize || max <= 0 {
		max = MaxListSize
	}
	processList, err := r.scrutinizer.ProcessList(
		request.EntityId,
		request.From,
		max,
		request.SearchTerm,
		request.Namespace,
		request.SrcNetId,
		request.Status,
		request.WithResults)
	if err != nil {
		return nil, fmt.Errorf("cannot get process list: (%s)", err)
	}
	for _, p := range processList {
		response.ProcessList = append(response.ProcessList, fmt.Sprintf("%x", p))
	}
	if len(response.ProcessList) == 0 {
		response.Message = "no processes found for the query"
		return &response, nil
	}

	response.Size = new(int64)
	*response.Size = int64(len(response.ProcessList))
	return &response, nil
}

func (r *RPCAPI) getProcessInfo(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	var err error
	response.Process, err = r.scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		return nil, fmt.Errorf("cannot get process info: (%v)", err)
	}
	return &response, nil
}

func (r *RPCAPI) getProcessSummary(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	if len(request.ProcessID) != types.ProcessIDsize {
		return nil, fmt.Errorf("cannot get envelope status: (malformed processId)")
	}

	// Get process info
	procInfo, err := r.scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		return nil, fmt.Errorf("cannot get envelope status: (cannot get process info (%v))", err)
	}

	// Get total number of votes (including invalid/null)
	eh, err := r.scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		response.Message = fmt.Sprintf("cannot get envelope height: %v", err)
		return &response, nil
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
	return &response, nil
}

func (r *RPCAPI) getProcessCount(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	response.Size = new(int64)
	count := r.scrutinizer.ProcessCount(request.EntityId)
	*response.Size = int64(count)
	return &response, nil
}

func (r *RPCAPI) getEntityCount(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	response.Size = new(int64)
	*response.Size = int64(r.scrutinizer.EntityCount())
	return &response, nil
}

func (r *RPCAPI) getProcessKeys(request *api.APIrequest) (*api.APIresponse, error) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		return nil, fmt.Errorf("cannot get envelope status: (malformed processId)")
	}
	process, err := r.vocapp.State.Process(request.ProcessID, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get process encryption public keys: (%s)", err)
	}
	var response api.APIresponse
	var pubs, privs []api.Key
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
	response.EncryptionPublicKeys = pubs
	response.EncryptionPrivKeys = privs
	return &response, nil
}

func (r *RPCAPI) getResultsWeight(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	w, err := r.scrutinizer.GetResultsWeight(request.ProcessID)
	if err != nil {
		return nil, fmt.Errorf("cannot get results weight: %v", err)
	}
	response.Weight = w.String()
	return &response, nil
}

func (r *RPCAPI) getOracleResults(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	if len(request.ProcessID) != types.ProcessIDsize {
		return nil, fmt.Errorf("cannot get oracle results: (malformed processId)")
	}
	var err error
	response.Results, err = r.vocapp.State.GetProcessResults(request.ProcessID)
	if err != nil {
		return nil, fmt.Errorf("cannot get oracle results: %v", err)
	}
	return &response, nil
}

func (r *RPCAPI) getResults(request *api.APIrequest) (*api.APIresponse, error) {
	if len(request.ProcessID) != types.ProcessIDsize {
		return nil, fmt.Errorf("cannot get results: (malformed processId)")
	}
	var response api.APIresponse
	// Get process info
	procInfo, err := r.scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		return nil, fmt.Errorf("cannot get results: (%v)", err)
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
	vr, err := r.scrutinizer.GetResults(request.ProcessID)
	if err != nil && err != scrutinizer.ErrNoResultsYet {
		return nil, fmt.Errorf("cannot get results: (%v)", err)
	}
	if errors.Is(err, scrutinizer.ErrNoResultsYet) {
		response.Message = scrutinizer.ErrNoResultsYet.Error()
		return &response, nil
	}
	if vr == nil {
		return nil, fmt.Errorf("cannot get results: (unknown error fetching results)")
	}
	response.Results = scrutinizer.GetFriendlyResults(vr.Votes)
	response.Final = &vr.Final
	h := uint32(vr.EnvelopeHeight)
	response.Height = &h
	// Get total number of votes (including invalid/null)
	eh, err := r.scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		response.Message = fmt.Sprintf("cannot get envelope height: %v", err)
		return &response, nil
	}
	votes := uint32(eh)
	response.Height = &votes
	response.Weight = vr.Weight.String()
	return &response, nil
}

// known entities
func (r *RPCAPI) getEntityList(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	if request.ListSize > MaxListSize || request.ListSize <= 0 {
		request.ListSize = MaxListSize
	}
	response.EntityIDs = r.scrutinizer.EntityList(request.ListSize, request.From, request.SearchTerm)
	return &response, nil
}

func (r *RPCAPI) getBlockStatus(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	h := r.vocapp.Height()
	response.Height = &h
	response.BlockTime = r.vocinfo.BlockTimes()
	response.BlockTimestamp = int32(r.vocapp.Timestamp())
	return &response, nil
}
