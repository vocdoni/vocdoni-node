package router

import (
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const MaxListSize = 64

func (r *Router) submitRawTx(request routerRequest) {
	res, err := r.vocapp.SendTx(request.Payload)
	if err != nil {
		r.sendError(request, err.Error())
		return
	}
	if res == nil {
		r.sendError(request, "no reply from Vochain")
		return
	}
	if res.Code != 0 {
		r.sendError(request, string(res.Data))
		return
	}
	log.Debugf("broadcasting tx hash:%s", res.Hash)
	var response types.MetaResponse
	response.Payload = fmt.Sprintf("%x", res.Data) // return nullifier or other info
	if err = request.Send(r.buildReply(request, &response)); err != nil {
		log.Warnf("error sending raw tx: %v", err)
	}
}

func (r *Router) submitEnvelope(request routerRequest) {
	var err error
	if request.Payload == nil {
		r.sendError(request, "payload is empty")
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
		r.sendError(request, fmt.Sprintf("cannot marshal vote transaction: (%s)", err))
		return
	}

	res, err := r.vocapp.SendTx(txBytes)
	if err != nil || res == nil {
		r.sendError(request, fmt.Sprintf("cannot broadcast transaction: (%s)", err))
		return
	}

	// Get mempool checkTx reply
	if res.Code != 0 {
		r.sendError(request, string(res.Data))
		return
	}
	log.Infof("broadcasting vochain tx hash:%s code:%d", res.Hash, res.Code)
	var response types.MetaResponse
	response.Nullifier = fmt.Sprintf("%x", res.Data)
	if err = request.Send(r.buildReply(request, &response)); err != nil {
		log.Warnf("error on submitEnvelope: %v", err)
	}
}

func (r *Router) getStats(request routerRequest) {
	var err error
	stats := new(types.VochainStats)
	stats.BlockHeight = r.vocapp.Height()
	stats.BlockTimeStamp = int32(r.vocapp.State.Header(true).Timestamp)
	stats.EntityCount = r.Scrutinizer.EntityCount()
	if stats.EnvelopeCount, err = r.Scrutinizer.GetEnvelopeHeight([]byte{}); err != nil {
		log.Warnf("could not count vote envelopes: %s", err)
	}
	stats.ProcessCount = r.Scrutinizer.ProcessCount([]byte{})
	vals, _ := r.vocapp.State.Validators(true)
	stats.ValidatorCount = len(vals)
	stats.BlockTime = r.vocinfo.BlockTimes()
	stats.ChainID = r.vocapp.Node.GenesisDoc().ChainID
	stats.GenesisTimeStamp = r.vocapp.Node.GenesisDoc().GenesisTime
	stats.Syncing = r.vocapp.IsSynchronizing()

	var response types.MetaResponse
	response.Stats = stats
	if err != nil {
		log.Errorf("could not marshal vochainStats: %s", err)
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelopeStatus(request routerRequest) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		r.sendError(request, "cannot get envelope status: (malformed processId)")
		return
	}
	// check nullifier
	if len(request.Nullifier) != types.VoteNullifierSize {
		r.sendError(request, "cannot get envelope status: (malformed nullifier)")
		return
	}

	// Check envelope status and send reply
	var response types.MetaResponse
	response.Registered = types.False
	vr, err := r.Scrutinizer.GetEnvelopeReference(request.Nullifier)
	if err != nil {
		if err == scrutinizer.ErrNotFoundInDatabase {
			request.Send(r.buildReply(request, &response))
			return
		}
		r.sendError(request, fmt.Sprintf("cannot get envelope status: (%v)", err))
		return
	}
	response.Registered = types.True
	response.Height = &vr.Height
	response.BlockTimestamp = int32(vr.CreationTime.Unix())
	response.ProcessID = vr.ProcessID
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelope(request routerRequest) {
	// check nullifier
	if len(request.Nullifier) != types.VoteNullifierSize {
		r.sendError(request, "cannot get envelope: (malformed nullifier)")
		return
	}
	env, _, err := r.Scrutinizer.GetEnvelope(request.Nullifier)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get envelope: (%v)", err))
		return
	}
	var response types.MetaResponse
	response.Envelope = env
	response.Registered = types.True
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelopeHeight(request routerRequest) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize && len(request.ProcessID) != 0 {
		r.sendError(request, "cannot get envelope height: (malformed processId)")
		return
	}
	votes, err := r.Scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get envelope height: (%v)", err))
		return
	}

	var response types.MetaResponse
	response.Height = new(uint32)
	*response.Height = uint32(votes)
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockHeight(request routerRequest) {
	var response types.MetaResponse
	h := r.vocapp.Height()
	response.Height = &h
	response.BlockTimestamp = int32(r.vocapp.Timestamp())
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessList(request routerRequest) {
	var response types.MetaResponse
	max := request.ListSize
	if max > MaxListSize || max <= 0 {
		max = MaxListSize
	}
	processList, err := r.Scrutinizer.ProcessList(
		request.EntityId,
		request.SearchTerm,
		request.Namespace,
		request.Status,
		request.WithResults,
		request.From,
		max)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get process list: (%s)", err))
		return
	}
	for _, p := range processList {
		response.ProcessList = append(response.ProcessList, fmt.Sprintf("%x", p))
	}
	if len(response.ProcessList) == 0 {
		response.Message = "no processes found for the query"
		request.Send(r.buildReply(request, &response))
		return
	}

	response.Size = new(int64)
	*response.Size = int64(len(response.ProcessList))
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessInfo(request routerRequest) {
	var response types.MetaResponse
	var err error
	response.Process, err = r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get process info: %v", err))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessCount(request routerRequest) {
	var response types.MetaResponse
	response.Size = new(int64)
	count := r.Scrutinizer.ProcessCount(request.EntityId)
	*response.Size = count
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEntityCount(request routerRequest) {
	var response types.MetaResponse
	response.Size = new(int64)
	*response.Size = r.Scrutinizer.EntityCount()
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getProcessKeys(request routerRequest) {
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		r.sendError(request, "cannot get envelope status: (malformed processId)")
		return
	}
	process, err := r.vocapp.State.Process(request.ProcessID, true)
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
	// check pid
	if len(request.ProcessID) != types.ProcessIDsize {
		r.sendError(request, "cannot get envelope status: (malformed processId)")
		return
	}
	if request.ListSize > MaxListSize {
		r.sendError(request, fmt.Sprintf("listSize overflow, maximum is %d", MaxListSize))
		return
	}
	if request.ListSize == 0 {
		request.ListSize = MaxListSize
	}
	var response types.MetaResponse
	var err error
	response.Envelopes, err = r.Scrutinizer.GetEnvelopes(request.ProcessID)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get envelope list: (%s)", err))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getValidatorList(request routerRequest) {
	var response types.MetaResponse
	var err error
	response.ValidatorList, err = r.vocapp.State.Validators(true)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get validator list: %v", err))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlock(request routerRequest) {
	var response types.MetaResponse
	if request.BlockHeight > r.vocapp.Height() {
		r.sendError(request, fmt.Sprintf("block height %d not valid for vochain with height %d", request.BlockHeight, r.vocapp.Height()))
		return
	}
	response.Block = r.Scrutinizer.App.GetBlockByHeight(int64(request.BlockHeight))
	if response.Block == nil {
		r.sendError(request, fmt.Sprintf("cannot get block: no block with height %d", request.BlockHeight))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockByHash(request routerRequest) {
	var response types.MetaResponse
	response.Block = r.Scrutinizer.App.GetBlockByHash(request.Payload)
	if response.Block == nil {
		r.sendError(request, fmt.Sprintf("cannot get block: no block with hash %x", request.Payload))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockList(request routerRequest) {
	var response types.MetaResponse
	for i := 0; i < request.ListSize; i++ {
		if int64(request.From)+int64(i) > r.vocinfo.Height() {
			break
		}
		response.BlockList = append(response.BlockList, r.Scrutinizer.App.GetBlockByHeight(int64(request.From)+int64(i)))
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getTx(request routerRequest) {
	var response types.MetaResponse
	tx, hash, err := r.Scrutinizer.App.GetTxHash(request.BlockHeight, request.TxIndex)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get tx: %v", err))
		return
	}
	response.Tx = &types.TxPackage{
		Tx:          tx.Tx,
		BlockHeight: request.BlockHeight,
		Index:       request.TxIndex,
		Hash:        hash,
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getTxListForBlock(request routerRequest) {
	var response types.MetaResponse
	block := r.vocapp.Node.BlockStore().LoadBlock(int64(request.BlockHeight))
	if block == nil {
		r.sendError(request, fmt.Sprintf("cannot get tx list: block does not exist"))
		return
	}
	if request.ListSize > MaxListSize || request.ListSize <= 0 {
		request.ListSize = MaxListSize
	}
	maxIndex := request.From + request.ListSize
	for i := request.From; i < maxIndex && i < len(block.Txs); i++ {
		signedTx := new(models.SignedTx)
		proto.Unmarshal(block.Txs[i], signedTx)
		response.TxList = append(response.TxList, &types.TxPackage{
			Tx:          signedTx.Tx,
			BlockHeight: request.BlockHeight,
			Index:       int32(i),
			Hash:        tmtypes.Tx(block.Txs[i]).Hash(),
		})
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getResultsWeight(request routerRequest) {
	var response types.MetaResponse
	w, err := r.Scrutinizer.GetResultsWeight(request.ProcessID)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get results weight: %v", err))
		return
	}
	response.Weight = w.String()
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getResults(request routerRequest) {
	if len(request.ProcessID) != types.ProcessIDsize {
		r.sendError(request, "cannot get envelope status: (malformed processId)")
		return
	}

	var response types.MetaResponse

	// Get process info
	procInfo, err := r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		r.sendError(request, err.Error())
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
		r.sendError(request, err.Error())
		return
	}
	if err == scrutinizer.ErrNoResultsYet {
		response.Message = scrutinizer.ErrNoResultsYet.Error()
		request.Send(r.buildReply(request, &response))
		return
	}
	if vr == nil {
		r.sendError(request, "unknown problem fetching results")
		return
	}
	response.Results = scrutinizer.GetFriendlyResults(vr.Votes)
	response.Final = &vr.Final
	// Get total number of votes (including invalid/null)
	eh, err := r.Scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		response.Message = fmt.Sprintf("cannot get envelope height: %v", err)
		request.Send(r.buildReply(request, &response))
		return
	}
	votes := uint32(eh)
	response.Height = &votes
	response.Weight = vr.Weight.String()
	request.Send(r.buildReply(request, &response))
}

// known entities
func (r *Router) getEntityList(request routerRequest) {
	var response types.MetaResponse
	if request.ListSize > MaxListSize || request.ListSize <= 0 {
		request.ListSize = MaxListSize
	}
	response.EntityIDs = r.Scrutinizer.EntityList(request.SearchTerm, request.ListSize, request.From)
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockStatus(request routerRequest) {
	var response types.MetaResponse
	h := r.vocapp.Height()
	response.Height = &h
	response.BlockTime = r.vocinfo.BlockTimes()
	response.BlockTimestamp = int32(r.vocapp.Timestamp())
	request.Send(r.buildReply(request, &response))
}
