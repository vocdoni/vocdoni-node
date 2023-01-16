package apiclient

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// CensusProof represents proof for a voter in a census.
type CensusProof struct {
	Proof   types.HexBytes
	Value   types.HexBytes
	Weight  uint64
	KeyType models.ProofArbo_KeyType
}

type CensusProofZk struct {
	CircuitParametersIndex int32
	Proof                  types.HexBytes
	PubSignals             types.HexBytes
	Weight                 uint64
	KeyType                models.ProofArbo_KeyType
	Nullifier              types.HexBytes
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
		cp.Weight = censusData.Weight.ToInt().Uint64()
	} else {
		cp.Weight = 1
	}
	return &cp, nil
}

// CensusGenProofZk function generates the census proof of a election based on
// ZkSnarks. It uses the current apiclient circuit config to instance the
// circuit and generates the proof for the censusRoot, electionId and voter
// babyjubjub private key provided.
func (c *HTTPclient) CensusGenProofZk(censusRoot, electionID types.HexBytes, privVoterKey babyjub.PrivateKey) (*CensusProofZk, error) {
	// Get BabyJubJub key from current client
	pubVoterKey, err := c.BabyJubJubPriv2PubKey(privVoterKey)
	if err != nil {
		return nil, err
	}

	strPrivateKey := babyjub.SkToBigInt(&privVoterKey).String()
	// Get merkle proof associated to the voter key provided, that will contains
	// the leaf siblings and value (weight)
	resp, code, err := c.Request("GET", nil, "censuses", censusRoot.String(), "proof", pubVoterKey.String())
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

	log.Debugw("zk census data received, starting to generate the proof inputs...", map[string]interface{}{
		"censusRoot": censusRoot.String(), "electionId": electionID.String()})

	// Encode census root
	strCensusRoot := arbo.BytesToBigInt(censusRoot).String()
	// Get vote weight
	weight := new(big.Int).SetInt64(1)
	if censusData.Weight != nil {
		weight = censusData.Weight.ToInt()
	}
	// Get nullifier and encoded processId
	nullifier, strProcessId, err := c.GetNullifierZk(privVoterKey, electionID)
	if err != nil {
		return nil, err
	}
	strNullifier := new(big.Int).SetBytes(nullifier).String()
	// Calculate and encode vote hash -> sha256(voteWeight)
	voteHash := sha256.Sum256(censusData.Value)
	strVoteHash := []string{
		new(big.Int).SetBytes(arbo.SwapEndianness(voteHash[:16])).String(),
		new(big.Int).SetBytes(arbo.SwapEndianness(voteHash[16:])).String(),
	}
	// Unpack and encode siblings
	unpackedSiblings, err := arbo.UnpackSiblings(arbo.HashFunctionPoseidon, censusData.Proof)
	if err != nil {
		return nil, fmt.Errorf("error unpacking merkle tree proof: %w", err)
	}
	// Create a list of siblings with the same number of items that levels
	// allowed by the circuit (from its config) plus one. Fill with zeros if its
	// needed.
	strSiblings := make([]string, c.circuit.levels+1)
	for i := 0; i < len(strSiblings); i++ {
		newSibling := "0"
		if i < len(unpackedSiblings) {
			newSibling = arbo.BytesToBigInt(unpackedSiblings[i]).String()
		}
		strSiblings[i] = newSibling
	}
	// Get artifacts of the current circuit
	circuit, err := circuit.LoadZkCircuit(context.Background(), c.circuit.conf)
	if err != nil {
		return nil, fmt.Errorf("error loading circuit: %w", err)
	}
	log.Debugw("zk circuit loaded", map[string]interface{}{
		"censusRoot": censusRoot.String(), "electionId": electionID.String(),
		"circuitURI": c.circuit.conf.URI})
	// Create the inputs and encode them into a JSON
	rawInputs := map[string]interface{}{
		"censusRoot":     strCensusRoot,
		"censusSiblings": strSiblings,
		"weight":         weight.String(),
		"privateKey":     strPrivateKey,
		"voteHash":       strVoteHash,
		"processId":      strProcessId,
		"nullifier":      strNullifier,
	}
	inputs, err := json.Marshal(rawInputs)
	if err != nil {
		return nil, fmt.Errorf("error encoding inputs: %w", err)
	}
	log.Debugw("proof inputs generated", map[string]interface{}{
		"censusRoot": censusRoot.String(), "electionId": electionID.String(),
		"nullifier": strNullifier})
	// Calculate the proof for the current apiclient circuit config and the
	// inputs encoded.
	proof, err := prover.Prove(circuit.ProvingKey, circuit.Wasm, inputs)
	if err != nil {
		return nil, err
	}
	// Encode the results as bytes and return the proof
	encProof, encPubSignals, err := proof.Bytes()
	if err != nil {
		return nil, err
	}
	return &CensusProofZk{
		CircuitParametersIndex: ZKCircuitIndex,
		Proof:                  encProof,
		PubSignals:             encPubSignals,
		Weight:                 weight.Uint64(),
		KeyType:                models.ProofArbo_PUBKEY,
		Nullifier:              nullifier,
	}, nil
}
