package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/indexer"
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
		"/votes/{voteID}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.getVoteHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/votes/verify/{electionID}/{voteID}",
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
//	@Router					/votes/{voteID} [get]
func (a *API) getVoteHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("voteID")))
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

	// If VotePackage is valid JSON, it's not encrypted, so we can include it.
	if json.Valid(voteData.VotePackage) {
		vote.VotePackage = voteData.VotePackage
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
//	@Param			electionID	path	string	true	"Election id"
//	@Param			voteID		path	string	true	"Nullifier of the vote"
//	@Success		200			"(empty body)"
//	@Failure		404			{object}	apirest.APIerror
//	@Router			/votes/verify/{electionID}/{voteID} [get]
func (a *API) verifyVoteHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("voteID")))
	if err != nil {
		return ErrCantParseVoteID.WithErr(err)
	}
	if len(voteID) == 0 {
		return ErrVoteIDMalformed.Withf("%x", voteID)
	}
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam("electionID"), err)
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
