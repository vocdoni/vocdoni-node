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

// VoteData contains the data needed to create a vote.
//
// Choices is a list of choices, where each position represents a question.
// ElectionID is the ID of the election.
// ProofTree is the proof tree of the vote.
//
// KeyType is the type of the key used when the census was created. It can be
// either models.ProofArbo_ADDRESS or models.ProofArbo_PUBKEY (default).
type VoteData struct {
	Choices    []int
	ElectionID types.HexBytes
	ProofTree  *CensusProof
	KeyType    models.ProofArbo_KeyType
}

// Vote sends a vote to the Vochain. The vote is a VoteData struct,
// which contains the electionID, the choices and the proofTree.  The
// return value is the voteID (nullifier).
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
					Siblings: v.ProofTree.Proof,
					Value:    v.ProofTree.Value,
					KeyType:  v.KeyType,
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
	resp, code, err := c.Request("POST", voteAPI, "votes")
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
