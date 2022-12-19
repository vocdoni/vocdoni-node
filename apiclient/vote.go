package apiclient

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/zk/prover"
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
}

var ErrNotAnonymousVoteInfo = fmt.Errorf("the current vote data not contain enought info to perform an anonymous vote")

// Vote sends a vote to the Vochain. The vote is a VoteData struct,
// which contains the electionID, the choices and the proof.  The
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

	// Get census type
	election, err := c.Election(v.ElectionID)
	if err != nil {
		return nil, err
	}

	// Build the proof
	switch {
	case election.VoteMode.Anonymous:
		if v.ProofMkTree == nil {
			return nil, ErrNotAnonymousVoteInfo
		}
		// zksnark
		if vote.Proof, err = c.GenerateZkProof(election, v); err != nil {
			return nil, err
		}
	case v.ProofCSP != nil:
		p := models.ProofCA{}
		if err := proto.Unmarshal(v.ProofCSP, &p); err != nil {
			return nil, fmt.Errorf("could not decode CSP proof: %w", err)
		}
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Ca{Ca: &p},
		}
	default:
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
	if code != bearerstdapi.HTTPstatusCodeOK {
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

func (c *HTTPclient) GenerateZkProof(election *api.Election, v *VoteData) (*models.Proof, error) {
	var proofInputs = struct {
		CensusRoot     []byte   `json:"censusRoot"`
		CensusSiblings []string `json:"censusSiblings"`
		Weight         string   `json:"weight"`
		PrivateKey     string   `json:"privateKey"`
		VoteHash       []string `json:"voteHash"`
		ProcessId      []string `json:"processId"`
		Nullifier      string   `json:"nullifier"`
	}{
		CensusRoot: election.Census.CensusRoot,
		Weight:     new(big.Int).SetBytes(v.ProofMkTree.Value).String(),
	}

	// Get siblings
	siblings, err := arbo.UnpackSiblings(arbo.HashFunctionPoseidon, v.ProofMkTree.Proof)
	if err != nil {
		return nil, err
	}

	for _, sibling := range siblings {
		strSiblign := arbo.BytesToBigInt(sibling).String()
		proofInputs.CensusSiblings = append(proofInputs.CensusSiblings, strSiblign)
	}

	// Private key ?¿

	// Generate vote hash ?¿

	// Generate processId
	processHash := sha256.Sum256(v.ElectionID)
	proofInputs.ProcessId = []string{
		new(big.Int).SetBytes(arbo.SwapEndianness(processHash[:16])).String(),
		new(big.Int).SetBytes(arbo.SwapEndianness(processHash[16:])).String(),
	}

	// Nullifier ?¿

	// Marshal inputs
	inputs, err := json.Marshal(proofInputs)
	if err != nil {
		return nil, err
	}

	// Get artifacts (zkey & wasm)
	zkey, wasm := []byte{}, []byte{}

	proof, pubSignals, err := prover.Prove(zkey, wasm, inputs)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
