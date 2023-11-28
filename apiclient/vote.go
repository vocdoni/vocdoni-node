package apiclient

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// VoteData contains the data needed to create a vote.
//
// Choices is a list of choices, where each position represents a question.
// ElectionID is the ID of the election.
// VoteWeight is the desired weight for voting. It can be less than or equal
// to the  weight registered in the census. If is defined as nil, it will be
// equal to the registered one.
// ProofMkTree is the proof of the vote for an off chain tree, weighted election.
// ProofCSP is the proof of the vote for a CSP election.
//
// KeyType is the type of the key used when the census was created. It can be
// either models.ProofArbo_ADDRESS (default) or models.ProofArbo_PUBKEY
// (deprecated).
type VoteData struct {
	Choices    []int
	Election   *api.Election
	VoteWeight *big.Int

	ProofMkTree  *CensusProof
	ProofSIKTree *CensusProof
	ProofCSP     types.HexBytes
	Keys         []api.Key

	// if VoterAccount is set, it will be used to sign the vote
	// instead of the keys found in HTTPclient.account
	VoterAccount *ethereum.SignKeys
}

// Vote sends a vote to the Vochain. The vote is a VoteData struct,
// which contains the electionID, the choices and the proof.
// if VoterAccount is set, it's used to sign the vote, else it defaults
// to signing with the account set in HTTPclient.
// The return value is the voteID (nullifier).
func (cl *HTTPclient) Vote(v *VoteData) (types.HexBytes, error) {
	c := cl
	if v.VoterAccount != nil {
		c = cl.Clone(hex.EncodeToString(v.VoterAccount.PrivateKey()))
	}

	var vote *models.VoteEnvelope
	var err error

	if v.Keys != nil {
		vote, err = c.voteEnvelopeWithKeys(v.Choices, v.Keys, v.Election)
	} else {
		vote, err = c.prepareVoteEnvelope(v.Choices, v.Election)
	}
	if err != nil {
		return nil, err
	}

	log.Debugw("generating a new vote", "electionId", v.Election.ElectionID, "voter", c.account.AddressString())
	voteAPI := &api.Vote{}
	censusOriginCSP := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_CA)]
	censusOriginWeighted := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED)]
	switch {
	case v.Election.VoteMode.Anonymous:
		// support no vote weight provided
		if v.VoteWeight == nil {
			v.VoteWeight = v.ProofMkTree.LeafWeight
		}
		// generate circuit inputs with the election, census and voter
		// information and encode it into a json
		rawInputs, err := circuit.GenerateCircuitInput(circuit.CircuitInputsParameters{
			Account:         c.account,
			ElectionId:      v.Election.ElectionID,
			CensusRoot:      v.Election.Census.CensusRoot,
			SIKRoot:         v.ProofSIKTree.Root,
			CensusSiblings:  v.ProofMkTree.Siblings,
			SIKSiblings:     v.ProofSIKTree.Siblings,
			VoteWeight:      v.VoteWeight,
			AvailableWeight: v.ProofMkTree.LeafWeight,
		})
		if err != nil {
			return nil, fmt.Errorf("could not generate zk circuit inputs: %w", err)
		}
		inputs, err := json.Marshal(rawInputs)
		if err != nil {
			return nil, fmt.Errorf("error encoding inputs: %w", err)
		}
		// instance the prover with the circuit config loaded and generate the
		// proof for the calculated inputs
		proof, err := prover.Prove(c.circuit.ProvingKey, c.circuit.Wasm, inputs)
		if err != nil {
			return nil, fmt.Errorf("could not generate anonymous proof: %w", err)
		}
		// encode the proof into a protobuf
		protoProof, err := zk.ProverProofToProtobufZKProof(proof, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		// include vote nullifier and the encoded proof in a VoteEnvelope
		nullifier, err := proof.ExtractPubSignal("nullifier")
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
			return nil, fmt.Errorf("could not prepare vote transaction: %w", err)
		}
	case v.Election.Census.CensusOrigin == censusOriginWeighted:
		// support custom vote weight
		var voteWeight []byte
		if v.VoteWeight != nil {
			voteWeight = v.VoteWeight.Bytes()
		}

		// copy the census proof in a VoteEnvelope
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:            models.ProofArbo_BLAKE2B,
					Siblings:        v.ProofMkTree.Proof,
					AvailableWeight: v.ProofMkTree.LeafValue,
					KeyType:         v.ProofMkTree.KeyType,
					VoteWeight:      voteWeight,
				},
			},
		}
		// prepare a signed vote transaction with the VoteEnvelope
		voteAPI, err = c.prepareVoteTx(vote, true)
		if err != nil {
			return nil, err
		}
	case v.Election.Census.CensusOrigin == censusOriginCSP:
		// decode the CSP proof and include in a VoteEnvelope
		p := models.ProofCA{}
		if err := proto.Unmarshal(v.ProofCSP, &p); err != nil {
			return nil, fmt.Errorf("could not decode CSP proof: %w", err)
		}
		vote.Proof = &models.Proof{
			Payload: &models.Proof_Ca{Ca: &p},
		}
		// prepare a signed vote transaction with the VoteEnvelope
		voteAPI, err = c.prepareVoteTx(vote, true)
		if err != nil {
			return nil, err
		}
	}
	// send the vote to the API and handle the response
	resp, code, err := c.Request(HTTPPOST, voteAPI, "votes")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	if err := json.Unmarshal(resp, &voteAPI); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %v", err)
	}
	// return the voteID received from the API as result of success vote
	return voteAPI.VoteID, nil
}

// Verify verifies a vote. The voteID is the nullifier of the vote.
func (c *HTTPclient) Verify(electionID, voteID types.HexBytes) (bool, error) {
	resp, code, err := c.Request(HTTPGET, nil, "votes", "verify", electionID.String(), voteID.String())
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

// prepareVoteEnvelope returns a models.VoteEnvelope struct with
// * a random Nonce
// * ProcessID set to the passed election
// * VotePackage with a plaintext or encrypted vochain.VotePackage
// * EncryptionKeyIndexes filled in, for encrypted votes
func (c *HTTPclient) prepareVoteEnvelope(choices []int, election *api.Election) (*models.VoteEnvelope, error) {
	var err error
	keysEnc := []api.Key{}

	if election.VoteMode.EncryptedVotes { // Get encryption keys
		keysEnc, err = c.EncryptionKeys(election.ElectionID)
		if err != nil {
			return nil, err
		}
	}

	return c.voteEnvelopeWithKeys(choices, keysEnc, election)
}

func (c *HTTPclient) voteEnvelopeWithKeys(choices []int, keysEnc []api.Key, election *api.Election) (*models.VoteEnvelope, error) {
	var keys []types.HexBytes
	var keyIndexes []uint32

	if election.VoteMode.EncryptedVotes {
		for _, k := range keysEnc {
			if len(k.Key) > 0 {
				keys = append(keys, k.Key)
				keyIndexes = append(keyIndexes, uint32(k.Index))
			}
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("no keys for election %x", election.ElectionID)
		}
	}

	// if EncryptedVotes is false, keys will be nil and prepareVotePackageBytes returns plaintext
	vpb, err := c.prepareVotePackageBytes(&state.VotePackage{Votes: choices}, keys)
	if err != nil {
		return nil, err
	}

	return &models.VoteEnvelope{
		Nonce:                util.RandomBytes(32),
		ProcessId:            election.ElectionID,
		VotePackage:          vpb,
		EncryptionKeyIndexes: keyIndexes,
	}, nil
}

// prepareVotePackageBytes returns a plaintext json.Marshal(vp) if keys is nil,
// else assigns a random hex string to vp.Nonce
// and encrypts the vp bytes for each given key as recipient
func (*HTTPclient) prepareVotePackageBytes(vp *state.VotePackage, keys []types.HexBytes) ([]byte, error) {
	if len(keys) > 0 {
		vp.Nonce = fmt.Sprintf("%x", util.RandomHex(32))
	}

	vpb, err := json.Marshal(vp)
	if err != nil {
		return nil, err
	}

	for i, k := range keys { // skipped if len(keys) == 0
		if len(k) == 0 {
			continue
		}
		log.Debugw("encrypting vote", "nonce", vp.Nonce, "key", k)
		pub, err := nacl.DecodePublic(k.String())
		if err != nil {
			return nil, fmt.Errorf("cannot decode encryption key with index %d: (%s)", i, err)
		}
		if vpb, err = nacl.Anonymous.Encrypt(vpb, pub); err != nil {
			return nil, fmt.Errorf("cannot encrypt: (%s)", err)
		}

	}

	return vpb, nil
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
