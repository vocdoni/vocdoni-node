package prover

import (
	"math/big"

	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/util"
)

// extractPubSignal decodes the requested public signal (identified by a string: "nullifier", "sikRoot", etc)
// from the current proof and returns it as a big.Int.
func (p *Proof) extractPubSignal(id string) (string, error) {
	// Check if the current proof contains public signals and it contains the
	// correct number of positions.
	if p.PubSignals == nil || len(p.PubSignals) != len(circuit.Global().Config.PublicSignals) {
		return "", ErrPublicSignalFormat
	}
	idx, found := circuit.Global().Config.PublicSignals[id]
	if !found {
		return "", ErrPubSignalNotFound
	}
	return p.PubSignals[idx], nil
}

// stringToBigInt converts a string into a standard big.Int.
func stringToBigInt(s string) (*big.Int, error) {
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, ErrParsingProofSignal
	}
	return i, nil
}

// ElectionID returns the ElectionID used as public input to generate the proof.
func (p *Proof) ElectionID() ([]byte, error) {
	electionID1str, err := p.extractPubSignal("electionId[0]")
	if err != nil {
		return nil, err
	}
	electionID2str, err := p.extractPubSignal("electionId[1]")
	if err != nil {
		return nil, err
	}
	return util.SplittedArboStrToBytes(electionID1str, electionID2str, false, true), nil
}

// VoteHash returns the VoteHash included into the current proof.
func (p *Proof) VoteHash() ([]byte, []string, error) {
	voteHash1str, err := p.extractPubSignal("voteHash[0]")
	if err != nil {
		return nil, nil, err
	}
	voteHash2str, err := p.extractPubSignal("voteHash[1]")
	if err != nil {
		return nil, nil, err
	}
	return util.SplittedArboStrToBytes(voteHash1str, voteHash2str, false, true), []string{
		voteHash1str,
		voteHash2str,
	}, nil
}

// CensusRoot returns the CensusRoot included into the current proof.
func (p *Proof) CensusRoot() ([]byte, error) {
	censusRoot, err := p.extractPubSignal("censusRoot")
	if err != nil {
		return nil, err
	}
	bi, err := stringToBigInt(censusRoot)
	if err != nil {
		return nil, err
	}
	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), bi), nil
}

// VoteWeight returns the VoteWeight included into the current proof.
func (p *Proof) VoteWeight() (*big.Int, error) {
	weight, err := p.extractPubSignal("voteWeight")
	if err != nil {
		return nil, err
	}
	return stringToBigInt(weight)
}

// AvailableWeight returns the AvailableWeight included into the current proof (not implemented).
func (p *Proof) AvailableWeight() (*big.Int, error) {
	panic("not implemented")
}

// Nullifier returns the Nullifier included into the current proof.
func (p *Proof) Nullifier() ([]byte, error) {
	nullifier, err := p.extractPubSignal("nullifier")
	if err != nil {
		return nil, err
	}
	bi, err := stringToBigInt(nullifier)
	if err != nil {
		return nil, err
	}
	return bi.Bytes(), err
}

// SIKRoot function returns the SIKRoot included into the current proof.
func (p *Proof) SIKRoot() ([]byte, error) {
	sikRoot, err := p.extractPubSignal("sikRoot")
	if err != nil {
		return nil, err
	}
	bi, err := stringToBigInt(sikRoot)
	if err != nil {
		return nil, err
	}
	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), bi), nil
}
