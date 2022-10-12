package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

const VoteHandler = "vote"

func (a *API) enableVoteHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/vote/submit",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		a.submitVoteHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/vote/{voteID}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.getVoteHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/vote/{voteID}/{electionID}/verify",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.verifyVoteHandler,
	); err != nil {
		return err
	}

	return nil
}

// /vote/submit
// submit a vote
func (a *API) submitVoteHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	req := &Vote{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
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
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /vote/<voteID>
// get a vote by its voteID (nullifier)
func (a *API) getVoteHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	voteID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("voteID")))
	if err != nil {
		return fmt.Errorf("cannot decode voteID: %w", err)
	}
	if len(voteID) != types.VoteNullifierSize {
		return fmt.Errorf("malformed voteId")
	}

	voteData, err := a.scrutinizer.GetEnvelope(voteID)
	if err != nil {
		return fmt.Errorf("cannot get vote: %w", err)
	}

	vote := &Vote{
		TxHash:               voteData.Meta.TxHash,
		VoteID:               voteData.Meta.Nullifier,
		EncryptionKeyIndexes: voteData.EncryptionKeyIndexes,
		VoteWeight:           voteData.Weight,
		VoteNumber:           &voteData.Meta.Height,
		ElectionID:           voteData.Meta.ProcessId,
		VoterID:              voteData.Meta.VoterID,
	}

	// check if votePackage is not encrypted
	if _, err := json.Marshal(voteData.VotePackage); err != nil {
		vote.VotePackage = string(voteData.VotePackage)
	}
	return nil
}

// /vote/<voteID>/<electionID>/verify
// verify a vote (get basic information)
func (a *API) verifyVoteHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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
	if ok, err := a.vocapp.State.EnvelopeExists(electionID, voteID, true); !ok || err != nil {
		return fmt.Errorf("not registered")
	}
	return nil
}
