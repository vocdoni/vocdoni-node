package apiclient

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// CensusProof represents proof for a voter in a census
type CensusProof struct {
	// Root contains census tree root
	Root types.HexBytes
	// Proof contains encoded and packed siblings as a census proof
	Proof types.HexBytes
	// LeafValue contains the associated value on the census tree
	LeafValue types.HexBytes
	// LeafWeight contains the decoded LeafValue as big.Int that represents
	// the associated weight for the voter.
	LeafWeight *big.Int
	// KeyType defines the type of the census tree key
	KeyType models.ProofArbo_KeyType
	// Siblings contains the decoded siblings keys
	Siblings []string
}

// NewCensus creates a new census and returns its ID. The censusType can be
// weighted (api.CensusTypeWeighted), zkweighted (api.CensusTypeZKWeighted) or
// csp (api.CensusTypeCSP).
func (c *HTTPclient) NewCensus(censusType string) (types.HexBytes, error) {
	// create a new census
	resp, code, err := c.Request(HTTPPOST, nil, "censuses", censusType)
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	if err := json.Unmarshal(resp, censusData); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return censusData.CensusID, nil
}

// CensusAddParticipants adds one or several participants to an existing census.
// The Key can be either the public key or address of the voter.
func (c *HTTPclient) CensusAddParticipants(censusID types.HexBytes, participants *api.CensusParticipants) error {
	resp, code, err := c.Request(HTTPPOST, &participants, "censuses", censusID.String(), "participants")
	if err != nil {
		return err
	}
	if code != apirest.HTTPstatusOK {
		return fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	return nil
}

// CensusSize returns the number of participants in a census.
func (c *HTTPclient) CensusSize(censusID types.HexBytes) (uint64, error) {
	resp, code, err := c.Request(HTTPGET, nil, "censuses", censusID.String(), "size")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	if err := json.Unmarshal(resp, censusData); err != nil {
		return 0, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return censusData.Size, nil
}

// CensusPublish publishes a census to the distributed data storage and returns its root hash
// and storage URI.
func (c *HTTPclient) CensusPublish(censusID types.HexBytes) (types.HexBytes, string, error) {
	resp, code, err := c.Request(HTTPPOST, nil, "censuses", censusID.String(), "publish", "async")
	if err != nil {
		return nil, "", err
	}
	if code != apirest.HTTPstatusOK {
		return nil, "", fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}

	censusData := &api.Census{}
	if err := json.Unmarshal(resp, censusData); err != nil {
		return nil, "", fmt.Errorf("could not unmarshal response: %w", err)
	}

	// wait for the census to be ready and get the root hash and storage URI
	for {
		time.Sleep(2 * time.Second)
		resp, code, err := c.Request(HTTPGET, nil, "censuses", censusData.CensusID.String(), "check")
		if err != nil {
			return nil, "", err
		}
		if code == apirest.HTTPstatusOK {
			if err := json.Unmarshal(resp, censusData); err != nil {
				return nil, "", fmt.Errorf("could not unmarshal response: %w", err)
			}
			break
		}
		if code == apirest.HTTPstatusNoContent {
			continue
		}
		return nil, "", fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	return censusData.CensusID, censusData.URI, nil
}

// CensusGenProof generates a proof for a voter in a census. The voterKey is the public key or address of the voter.
func (c *HTTPclient) CensusGenProof(censusID, voterKey types.HexBytes) (*CensusProof, error) {
	resp, code, err := c.Request(HTTPGET, nil, "censuses", censusID.String(), "proof", voterKey.String())
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	censusData := &api.Census{}
	if err := json.Unmarshal(resp, censusData); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	cp := CensusProof{
		Root:      censusData.CensusRoot,
		Proof:     censusData.CensusProof,
		LeafValue: censusData.Value,
		Siblings:  censusData.CensusSiblings,
	}
	if censusData.Weight != nil {
		cp.LeafWeight = censusData.Weight.MathBigInt()
	} else {
		cp.LeafWeight = new(big.Int).SetUint64(1)
	}
	return &cp, nil
}
