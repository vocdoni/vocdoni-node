package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/proto/build/go/models"
)

const VoteHandler = "votes"

func (a *API) enableVoteHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/votes",
		"POST",
		apirest.MethodAccessTypePublic,
		a.submitVoteHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/votes/{voteID}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.getVoteHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/votes/verify/{electionID}/{voteID}/",
		"GET",
		apirest.MethodAccessTypePublic,
		a.verifyVoteHandler,
	); err != nil {
		return err
	}

	return nil
}

// /votes
// submit a vote
func (a *API) submitVoteHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &Vote{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}

	// check if the transaction is of the correct type
	if ok, err := isTransactionType(req.TxPayload, &models.Tx_Vote{}); err != nil {
		return fmt.Errorf("could not check transaction type: %w", err)
	} else if !ok {
		return fmt.Errorf("transaction is not of type NewProcess")
	}

	// send the transaction to the mempool
	res, err := a.vocapp.SendTx(req.TxPayload)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("no reply from vochain")
	}
	if res.Code != 0 {
		return fmt.Errorf("%s", string(res.Data))
	}
	var data []byte
	if data, err = json.Marshal(Vote{VoteID: res.Data.Bytes(), TxHash: res.Hash.Bytes()}); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /votes/<voteID>
// get a vote by its voteID (nullifier)
func (a *API) getVoteHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("voteID")))
	if err != nil {
		return fmt.Errorf("cannot decode voteID: %w", err)
	}
	if len(voteID) != types.VoteNullifierSize {
		return fmt.Errorf("malformed voteId")
	}

	voteData, err := a.indexer.GetEnvelope(voteID)
	if err != nil {
		if errors.Is(err, indexer.ErrVoteNotFound) {
			return httprouter.ErrNotFound
		}
		return fmt.Errorf("cannot get vote: %w", err)
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

	// check if votePackage is not encrypted and if so, return it
	if _, err := json.Marshal(voteData.VotePackage); err == nil {
		vote.VotePackage = string(voteData.VotePackage)
	}

	var data []byte
	if data, err = json.Marshal(vote); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /votes/verify/<electionID>/<voteID>
// verify a vote (get basic information)
func (a *API) verifyVoteHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("voteID")))
	if err != nil {
		return fmt.Errorf("cannot decode voteID: %w", err)
	}
	if len(voteID) != types.VoteNullifierSize {
		return fmt.Errorf("malformed voteId")
	}
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil {
		return fmt.Errorf("cannot decode electionID: %w", err)
	}
	if len(voteID) != types.ProcessIDsize {
		return fmt.Errorf("malformed voteId")
	}
	if ok, err := a.vocapp.State.VoteExists(electionID, voteID, true); !ok || err != nil {
		return httprouter.ErrNotFound
	}
	return ctx.Send(nil, apirest.HTTPstatusCodeOK)
}
