package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
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
// ProofMkTree is the proof of the vote for a off chain tree, weighted election.
// ProofCSP is the proof of the vote fore a CSP election.
//
// KeyType is the type of the key used when the census was created. It can be
// either models.ProofArbo_ADDRESS or models.ProofArbo_PUBKEY (default).
type VoteData struct {
	Choices    []int
	ElectionID types.HexBytes

	ProofMkTree *CensusProof
	ProofCSP    types.HexBytes
	ProofZkTree *CensusProofZk
}

// Vote sends a vote to the Vochain. The vote is a VoteData struct,
// which contains the electionID, the choices and the proof. The
// return value is the fcvoteID (nullifier).
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

	// Get de election metadata
	election, err := c.Election(v.ElectionID)
	if err != nil {
		return nil, err
	}

	// Build the proof
	log.Debugw("generating a new vote", map[string]interface{}{"electionId": v.ElectionID})
	censusOriginCSP := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_CA)]
	censusOriginWeighted := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED)]
	switch {
	case election.VoteMode.Anonymous:
		// First check if the current vote mode configuration contains the flag
		// anonymouse setted to true, other weighted tree voting modes are
		// supported by the following case.
		log.Debugw("zk anonymous voting detected", map[string]interface{}{"electionId": v.ElectionID.String()})
		if v.ProofZkTree == nil {
			return nil, fmt.Errorf("no zk proof provided")
		}
		// Parse the provided proof and public signals using the prover parser
		// and encodes to a protobuf
		proof, err := prover.ParseProof(v.ProofZkTree.Proof, v.ProofZkTree.PubSignals)
		if err != nil {
			return nil, err
		}
		protoProof, err := zk.ProverProofToProtobufZKProof(proof, nil, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		log.Debugw("zk vote proof parsed from prover and encoded to protobuf", map[string]interface{}{
			"electionId": v.ElectionID.String()})

		// Set the result to the vote struct with the related nullifier
		vote.Nullifier = v.ProofZkTree.Nullifier
		vote.Proof = &models.Proof{
			Payload: &models.Proof_ZkSnark{
				ZkSnark: protoProof,
			},
		}
	case election.Census.CensusOrigin == censusOriginWeighted:
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: v.ProofMkTree.Proof,
					Value:    v.ProofMkTree.Value,
					KeyType:  v.ProofMkTree.KeyType,
				},
			},
		}
	case election.Census.CensusOrigin == censusOriginCSP:
		p := models.ProofCA{}
		if err := proto.Unmarshal(v.ProofCSP, &p); err != nil {
			return nil, fmt.Errorf("could not decode CSP proof: %w", err)
		}
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Ca{Ca: &p},
		}
	}

	// Sign and send the vote
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
	if code != apirest.HTTPstatusCodeOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	err = json.Unmarshal(resp, &voteAPI)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %v", err)
	}

	return voteAPI.VoteID, nil
}

// Verify verifies a vote. The voteID is the nullifier of the vote.
func (c *HTTPclient) Verify(electionID, voteID types.HexBytes) (bool, error) {
	resp, code, err := c.Request("GET", nil, "votes", "verify", electionID.String(), voteID.String())
	if err != nil {
		return false, err
	}
	if code == 200 {
		return true, nil
	}
	if code == 404 {
		return false, nil
	}
	return false, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
}
