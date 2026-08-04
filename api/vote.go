package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/indexer"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

const VoteHandler = "votes"

func (a *API) enableVoteHandlers() error {
	if err := a.Endpoint.RegisterMethod(
		"/votes",
		"POST",
		apirest.MethodAccessTypePublic,
		a.submitVoteHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/votes",
		"GET",
		apirest.MethodAccessTypePublic,
		a.votesListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/votes/{voteId}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.getVoteHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/votes/verify/{electionId}/{voteId}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.verifyVoteHandler,
	); err != nil {
		return err
	}

	return nil
}

// submitVoteHandler
//
//	@Summary		Submit a vote
//	@Description	Submit a vote using a protobuf signed transaction. The corresponding result are the vote id and transaction hash where the vote is registered.
//	@Tags			Votes
//	@Accept			json
//	@Produce		json
//	@Param			transaction	body		object{txPayload=string}	true	"Requires a protobuf signed transaction"
//	@Success		200			{object}	object{txHash=string, voteID=string}
//	@Router			/votes [post]
func (a *API) submitVoteHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &Vote{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}

	// check if the transaction is of the correct type
	if ok, err := isTransactionType[*models.Tx_Vote](req.TxPayload); err != nil {
		return ErrCantCheckTxType.WithErr(err)
	} else if !ok {
		return ErrTxTypeMismatch.Withf("expected Tx_Vote")
	}

	// send the transaction to the mempool
	res, err := a.sendTx(req.TxPayload)
	if err != nil {
		return err
	}

	data, err := json.Marshal(Vote{VoteID: res.Data.Bytes(), TxHash: res.Hash.Bytes()})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// getVoteHandler
//
//	@Summary				Get vote
//	@Description.markdown	getVoteHandler
//	@Tags					Votes
//	@Accept					json
//	@Produce				json
//	@Param					voteID	path		string	true	"Nullifier of the vote"
//	@Success				200		{object}	Vote
//	@Router					/votes/{voteId} [get]
func (a *API) getVoteHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam(ParamVoteId)))
	if err != nil {
		return ErrCantParseVoteID.WithErr(err)
	}
	if len(voteID) == 0 {
		return ErrVoteIDMalformed.Withf("%x", voteID)
	}

	voteData, err := a.indexer.GetEnvelope(voteID)
	if err != nil {
		if errors.Is(err, indexer.ErrVoteNotFound) {
			return ErrVoteNotFound
		}
		return ErrCantFetchEnvelope.WithErr(err)
	}

	vote := &Vote{
		TxHash:               voteData.Meta.TxHash,
		VoteID:               voteData.Meta.Nullifier,
		EncryptionKeyIndexes: voteData.EncryptionKeyIndexes,
		VoteWeight:           voteData.Weight,
		BlockHeight:          voteData.Meta.Height,
		ElectionID:           voteData.Meta.ProcessId,
		VoterID:              voteData.Meta.VoterID,
		TransactionIndex:     &voteData.Meta.TxIndex,
		OverwriteCount:       &voteData.OverwriteCount,
		Date:                 &voteData.Date,
	}

	// The memo is not indexed; read it from the authoritative state if present.
	// A read failure must not be reported as "this vote has no memo", so log it.
	if sdbVote, err := a.vocapp.State.Vote(voteData.Meta.ProcessId, voteData.Meta.Nullifier, true); err == nil {
		vote.Memo = sdbVote.GetMemo()
	} else if !errors.Is(err, vstate.ErrVoteNotFound) && !errors.Is(err, vstate.ErrProcessNotFound) {
		log.Warnw("could not read vote memo from state", "voteId", vote.VoteID.String(), "error", err.Error())
	}

	// If VotePackage is valid JSON, it's not encrypted, so we can include it direcectly.
	if json.Valid(voteData.VotePackage) {
		vote.VotePackage = voteData.VotePackage
	} else {
		// Otherwise, we need to decrypt it or include the encrypted version.
		process, err := a.vocapp.State.Process(voteData.Meta.ProcessId, true)
		if err != nil {
			return ErrCantFetchElection.WithErr(err)
		}
		// Try to decrypt the vote package
		var evp []byte
		if len(process.EncryptionPrivateKeys) >= len(voteData.EncryptionKeyIndexes) {
			evp, err = decryptVotePackage(voteData.VotePackage, process.EncryptionPrivateKeys, voteData.EncryptionKeyIndexes)
			if err != nil {
				log.Warnw("failed to decrypt vote package, skipping", "err", err)
			}
		}
		// If decryption failed, include the encrypted version
		if evp == nil {
			evp, err = json.Marshal(map[string][]byte{"encrypted": voteData.VotePackage})
			if err != nil {
				return ErrMarshalingJSONFailed
			}
		}
		vote.VotePackage = evp
	}

	var data []byte
	if data, err = json.Marshal(vote); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// verifyVoteHandler
//
//	@Summary		Verify vote
//	@Description	Check if vote is registered on the blockchain on specific election. Just return Ok status code
//	@Tags			Votes
//	@Accept			json
//	@Produce		json
//	@Param			electionId	path	string	true	"Election id"
//	@Param			voteId		path	string	true	"Nullifier of the vote"
//	@Success		200			"(empty body)"
//	@Failure		404			{object}	apirest.APIerror
//	@Router			/votes/verify/{electionId}/{voteId} [get]
func (a *API) verifyVoteHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam(ParamVoteId)))
	if err != nil {
		return ErrCantParseVoteID.WithErr(err)
	}
	if len(voteID) == 0 {
		return ErrVoteIDMalformed.Withf("%x", voteID)
	}
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam(ParamElectionId)))
	if err != nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam(ParamElectionId), err)
	}
	if len(voteID) != types.ProcessIDsize {
		return ErrVoteIDMalformed.Withf("%x", voteID)
	}
	// TODO: use the indexer to verify that a vote exists?
	if ok, err := a.vocapp.State.VoteExists(electionID, voteID, true); !ok || err != nil {
		return ErrVoteNotFound
	}
	return ctx.Send(nil, apirest.HTTPstatusOK)
}

// votesListHandler
//
//	@Summary		List votes
//	@Description	Returns the list of votes
//	@Tags			Votes
//	@Accept			json
//	@Produce		json
//	@Param			page		query		number	false	"Page"
//	@Param			limit		query		number	false	"Items per page"
//	@Param			electionId	query		string	false	"Election id"
//	@Success		200			{object}	VotesList
//	@Router			/votes [get]
func (a *API) votesListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseVoteParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamElectionId),
	)
	if err != nil {
		return err
	}

	list, err := a.votesList(params)
	if err != nil {
		return err
	}

	return marshalAndSend(ctx, list)
}

// votesList produces a filtered, paginated VotesList.
//
// Errors returned are always of type APIerror.
func (a *API) votesList(params *VoteParams) (*VotesList, error) {
	if params.ElectionID != "" && !a.indexer.ProcessExists(params.ElectionID) {
		return nil, ErrElectionNotFound
	}

	votes, total, err := a.indexer.VoteList(
		params.Limit,
		params.Page*params.Limit,
		params.ElectionID,
		"",
	)
	if err != nil {
		return nil, ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return nil, err
	}

	list := &VotesList{
		Votes:      []*Vote{},
		Pagination: pagination,
	}
	// Index the items by process so the memos below can be fetched with one state
	// lookup per process rather than one per row.
	byProcess := map[string][]int{}
	for _, vote := range votes {
		list.Votes = append(list.Votes, &Vote{
			ElectionID:       vote.ProcessId,
			VoteID:           vote.Nullifier,
			VoterID:          vote.VoterID,
			TxHash:           vote.TxHash,
			BlockHeight:      vote.Height,
			TransactionIndex: &vote.TxIndex,
			BlockTime:        vote.BlockTime,
		})
		byProcess[string(vote.ProcessId)] = append(byProcess[string(vote.ProcessId)], len(list.Votes)-1)
	}

	// The memo is not indexed; read it from the authoritative state, the same way
	// GET /votes/{voteId} does. State.Votes opens the votes subtree once per
	// process, so a page filtered by electionId costs a single open.
	for pid, rows := range byProcess {
		nullifiers := make([][]byte, len(rows))
		for i, row := range rows {
			nullifiers[i] = list.Votes[row].VoteID
		}
		sdbVotes, err := a.vocapp.State.Votes([]byte(pid), nullifiers, true)
		if err != nil {
			// Serve the page without memos rather than failing it, but say so: an
			// absent memo and a failed read must not look the same in the logs.
			log.Warnw("could not read vote memos from state", "processId",
				hex.EncodeToString([]byte(pid)), "error", err.Error())
			continue
		}
		for i, sdbVote := range sdbVotes {
			list.Votes[rows[i]].Memo = sdbVote.GetMemo()
		}
	}
	return list, nil
}

// parseVoteParams returns an VoteParams filled with the passed params
func parseVoteParams(paramPage, paramLimit, paramElectionID string) (*VoteParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	return &VoteParams{
		PaginationParams: pagination,
		ElectionID:       util.TrimHex(paramElectionID),
	}, nil
}
