package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type VoteData struct {
	Choices    []int
	ElectionID types.HexBytes
	ProofTree  *struct {
		Siblings types.HexBytes
		Value    types.HexBytes
	}
}

func (c *HTTPclient) Vote(v *VoteData) (types.HexBytes, error) {
	votePackage := &vochain.VotePackage{
		Votes: v.Choices,
	}
	votePackageBytes, err := json.Marshal(votePackage)
	if err != nil {
		return nil, err
	}
	vote := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(16),
		ProcessId:   v.ElectionID,
		VotePackage: votePackageBytes,
	}

	if v.ProofTree != nil {
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: v.ProofTree.Siblings,
					Value:    v.ProofTree.Value,
				},
			},
		}
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{
			Vote: vote,
		},
	})
	if err != nil {
		return nil, err
	}
	stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID())
	if err != nil {
		return nil, err
	}
	stxb, err := proto.Marshal(&stx)
	if err != nil {
		return nil, err
	}

	voteAPI := &api.Vote{TxPayload: stxb}
	resp, code, err := c.Request("POST", voteAPI, "vote", "submit")
	if err != nil {
		return nil, err
	}
	if code != bearerstdapi.HTTPstatusCodeOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	err = json.Unmarshal(resp, &voteAPI)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %v", err)
	}

	return voteAPI.VoteID, nil
}
