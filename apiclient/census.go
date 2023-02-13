package apiclient

import (
	"context"
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// CensusProof represents proof for a voter in a census.
type CensusProof struct {
	Proof   types.HexBytes
	Value   types.HexBytes
	Weight  types.BigInt
	KeyType models.ProofArbo_KeyType
}

// CensusProofZk represents the proof for a voter in a zk census. It contains
// the encoded proof and the circuit public tokens to verify it. It also keeps
// the weight of the vote and the key type of the merkle tree, and also includes
// the voter's nullifier. This last parameter is necessary because now, to
// compute the nullifier, the private key of the ZkAddress is required, so it
// cannot be computed by anyone but the voter.
type CensusProofZk struct {
	Proof      types.HexBytes
	PubSignals types.HexBytes
	Weight     types.BigInt
	KeyType    models.ProofArbo_KeyType
	Nullifier  types.HexBytes
}

// NewCensus creates a new census and returns its ID. The censusType can be
// weighted (api.CensusTypeWeighted) or zkindexed (api.CensusTypeZK).
func (c *HTTPclient) NewCensus(censusType string) (types.HexBytes, error) {
	// create a new census
	resp, code, err := c.Request("POST", nil, "censuses", censusType)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	err = json.Unmarshal(resp, censusData)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return censusData.CensusID, nil
}

// CensusAddParticipants adds one or several participants to an existing census.
// The Key can be either the public key or address of the voter.
func (c *HTTPclient) CensusAddParticipants(censusID types.HexBytes, participants *api.CensusParticipants) error {
	resp, code, err := c.Request("POST", &participants, "censuses", censusID.String(), "participants")
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	return nil
}

// CensusSize returns the number of participants in a census.
func (c *HTTPclient) CensusSize(censusID types.HexBytes) (uint64, error) {
	resp, code, err := c.Request("GET", nil, "censuses", censusID.String(), "size")
	if err != nil {
		return 0, err
	}
	if code != 200 {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	err = json.Unmarshal(resp, censusData)
	if err != nil {
		return 0, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return censusData.Size, nil
}

// CensusPublish publishes a census to the distributed data storage and returns its root hash
// and storage URI.
func (c *HTTPclient) CensusPublish(censusID types.HexBytes) (types.HexBytes, string, error) {
	resp, code, err := c.Request("POST", nil, "censuses", censusID.String(), "publish")
	if err != nil {
		return nil, "", err
	}
	if code != 200 {
		return nil, "", fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	err = json.Unmarshal(resp, censusData)
	if err != nil {
		return nil, "", fmt.Errorf("could not unmarshal response: %w", err)
	}
	return censusData.CensusID, censusData.URI, nil
}

// CensusGenProof generates a proof for a voter in a census. The voterKey is the public key or address of the voter.
func (c *HTTPclient) CensusGenProof(censusID, voterKey types.HexBytes) (*CensusProof, error) {
	resp, code, err := c.Request("GET", nil, "censuses", censusID.String(), "proof", voterKey.String())
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	err = json.Unmarshal(resp, censusData)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	cp := CensusProof{
		Proof: censusData.Proof,
		Value: censusData.Value,
	}
	if censusData.Weight != nil {
		cp.Weight = *censusData.Weight
	} else {
		cp.Weight = *new(types.BigInt).SetUint64(1)
	}
	return &cp, nil
}

// CensusGenProofZk function generates the census proof of a election based on
// ZkSnarks. It uses the current apiclient circuit config to instance the
// circuit and generates the proof for the censusRoot, electionId and voter
// babyjubjub private key provided.
func (c *HTTPclient) CensusGenProofZk(censusRoot, electionId types.HexBytes) (*CensusProofZk, error) {
	// Perform a request to get the type of the current census and check if it
	// is a zk census.
	resp, code, err := c.Request("GET", nil, "censuses", censusRoot.String(), "type")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	err = json.Unmarshal(resp, censusData)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}

	if censusData.Type != api.CensusTypeZKWeighted {
		return nil, fmt.Errorf("current census is not a zk census")
	}

	// Get merkle proof associated to the voter key provided, that will contains
	// the leaf siblings and value (weight)
	resp, code, err = c.Request("GET", nil, "censuses", censusRoot.String(), "proof", c.zkAddr.String())
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData = &api.Census{}
	err = json.Unmarshal(resp, censusData)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}

	log.Debugw("zk census data received, starting to generate the proof inputs...",
		"censusRoot", censusRoot.String(),
		"electionId", electionId.String(),
		"zkAddress", c.zkAddr.String())

	// get nullifier
	nullifier, err := c.zkAddr.Nullifier(electionId)
	if err != nil {
		return nil, err
	}

	// Encode census root
	strCensusRoot := arbo.BytesToBigInt(censusRoot).String()
	// Get vote weight
	weight := new(types.BigInt).SetUint64(1)
	if censusData.Weight != nil {
		weight = censusData.Weight
	}

	// Calculate and encode vote hash -> sha256(voteWeight)
	encElectionId := zk.BytesToArboStr(electionId)
	voteHash := zk.BytesToArboStr(weight.Bytes())

	// Create the inputs and encode them into a JSON
	rawInputs := map[string]interface{}{
		"censusRoot":     strCensusRoot,
		"censusSiblings": censusData.Siblings,
		"weight":         weight,
		"privateKey":     c.zkAddr.PrivKey.String(),
		"voteHash":       voteHash,
		"processId":      encElectionId,
		"nullifier":      nullifier.String(),
	}
	inputs, err := json.Marshal(rawInputs)
	if err != nil {
		return nil, fmt.Errorf("error encoding inputs: %w", err)
	}
	log.Debug("zk circuit proof inputs generated")

	// Get artifacts of the current circuit
	currentCircuit, err := circuit.LoadZkCircuit(context.Background(), c.circuit)
	if err != nil {
		return nil, fmt.Errorf("error loading circuit: %w", err)
	}
	log.Debugw("zk circuit loaded",
		"censusRoot", censusRoot.String(),
		"electionId", electionId.String(),
		"zkAddress", c.zkAddr.String())

	// Calculate the proof for the current apiclient circuit config and the
	// inputs encoded.
	proof, err := prover.Prove(currentCircuit.ProvingKey, currentCircuit.Wasm, inputs)
	if err != nil {
		return nil, err
	}
	// Encode the results as bytes and return the proof
	encProof, encPubSignals, err := proof.Bytes()
	if err != nil {
		return nil, err
	}
	return &CensusProofZk{
		Proof:      encProof,
		PubSignals: encPubSignals,
		Weight:     *weight,
		KeyType:    models.ProofArbo_PUBKEY,
		Nullifier:  nullifier.Bytes(),
	}, nil
}
