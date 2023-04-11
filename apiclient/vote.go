package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
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
// VotingWeight is the desired weight for voting. It can be less than or equal
// to the  weight registered in the census. If is defined as nil, it will be
// equal to the registered one.
// ProofMkTree is the proof of the vote for a off chain tree, weighted election.
// ProofCSP is the proof of the vote fore a CSP election.
//
// KeyType is the type of the key used when the census was created. It can be
// either models.ProofArbo_ADDRESS (default) or models.ProofArbo_PUBKEY
// (deprecated).
type VoteData struct {
	Choices      []int
	ElectionID   types.HexBytes
	VotingWeight *big.Int

	ProofMkTree *CensusProof
	ProofCSP    types.HexBytes
}

// Vote sends a vote to the Vochain. The vote is a VoteData struct,
// which contains the electionID, the choices and the proof. The
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

	// Get de election metadata
	election, err := c.Election(v.ElectionID)
	if err != nil {
		return nil, err
	}

	log.Debugw("generating a new vote", "electionId", v.ElectionID)
	voteAPI := &api.Vote{}
	censusOriginCSP := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_CA)]
	censusOriginWeighted := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED)]
	switch {
	case election.VoteMode.Anonymous:
		// support no vote weight provided
		if v.VotingWeight == nil {
			v.VotingWeight = v.ProofMkTree.LeafWeight
		}

		// generate circuit inputs with the election, census and voter
		// information and encode it into a json
		rawInputs, err := circuit.GenerateCircuitInput(c.zkAddr, election.Census.CensusRoot, election.ElectionID,
			v.ProofMkTree.LeafWeight, v.VotingWeight, v.ProofMkTree.Siblings)
		if err != nil {
			return nil, err
		}
		inputs, err := json.Marshal(rawInputs)
		if err != nil {
			return nil, fmt.Errorf("error encoding inputs: %w", err)
		}
		// load the correct circuit from the ApiClient configuration
		currentCircuit, err := circuit.LoadZkCircuit(context.Background(), c.circuit)
		if err != nil {
			return nil, fmt.Errorf("error loading circuit: %w", err)
		}
		// instance the prover with the circuit config loaded and generate the
		// proof for the calculated inputs
		proof, err := prover.Prove(currentCircuit.ProvingKey, currentCircuit.Wasm, inputs)
		if err != nil {
			return nil, err
		}
		// encode the proof into a protobuf
		protoProof, err := zk.ProverProofToProtobufZKProof(proof, nil, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		// include vote nullifier and the encoded proof in a VoteEnvelope
		nullifier, err := proof.Nullifier()
		if err != nil {
			return nil, err
		}
		vote.Nullifier = nullifier.Bytes()
		vote.Proof = &models.Proof{
			Payload: &models.Proof_ZkSnark{
				ZkSnark: protoProof,
			},
		}
		// prepare an unsigned vote transaction with the VoteEnvelope
		voteAPI, err = c.prepareVoteTx(vote, false)
		if err != nil {
			return nil, err
		}
	case election.Census.CensusOrigin == censusOriginWeighted:
		// support custom vote weight
		var votingWeight []byte
		if v.VotingWeight != nil {
			votingWeight = v.VotingWeight.Bytes()
		}

		// copy the census proof in a VoteEnvelope
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:         models.ProofArbo_BLAKE2B,
					Siblings:     v.ProofMkTree.Proof,
					LeafWeight:   v.ProofMkTree.LeafValue,
					KeyType:      v.ProofMkTree.KeyType,
					VotingWeight: votingWeight,
				},
			},
		}
		// prepare an signed vote transaction with the VoteEnvelope
		voteAPI, err = c.prepareVoteTx(vote, true)
		if err != nil {
			return nil, err
		}
	case election.Census.CensusOrigin == censusOriginCSP:
		// decode the CSP proof and include in a VoteEnvelope
		p := models.ProofCA{}
		if err := proto.Unmarshal(v.ProofCSP, &p); err != nil {
			return nil, fmt.Errorf("could not decode CSP proof: %w", err)
		}
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Ca{Ca: &p},
		}
		// prepare an signed vote transaction with the VoteEnvelope
		voteAPI, err = c.prepareVoteTx(vote, true)
		if err != nil {
			return nil, err
		}
	}
	// send the vote to the API and handle the response
	resp, code, err := c.Request("POST", voteAPI, "votes")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	err = json.Unmarshal(resp, &voteAPI)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %v", err)
	}
	// return the voteID received from the API as result of success vote
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

// prepareVoteTx prepare an api.Vote struct with the inner transactions encoded
// based on the vote provided and if it is signed or not.
func (c *HTTPclient) prepareVoteTx(vote *models.VoteEnvelope, signed bool) (*api.Vote, error) {
	// Encode vote transaction
	txPayload, err := proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{
			Vote: vote,
		},
	})
	if err != nil {
		return nil, err
	}
	stx := models.SignedTx{Tx: txPayload}

	// If it needs to be signed, sign the vote transaction
	if signed {
		stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID())
	}
	if err != nil {
		return nil, err
	}
	stxb, err := proto.Marshal(&stx)
	if err != nil {
		return nil, err
	}
	return &api.Vote{TxPayload: stxb}, nil
}
