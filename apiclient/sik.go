package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
)

type SikProof struct {
	// Proof contains encoded and packed siblings as a census proof
	Proof types.HexBytes
	// Root contains the sik merkle tree roor
	Root types.HexBytes
	// Siblings contains the decoded siblings keys
	Siblings []string
}

type SikRoots []string

// ValidSikRoots returns the currently valid roots of SIK merkle tree from the
// API.
func (c *HTTPclient) ValidSikRoots() (SikRoots, error) {
	resp, code, err := c.Request(HTTPGET, nil, "sik", "roots")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	sp := struct {
		SikRoots []string `json:"sikroots"`
	}{}
	if err := json.Unmarshal(resp, &sp); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return sp.SikRoots, nil
}

// SikGenProof generates a proof for the voter address in the sik merkle tree.
func (c *HTTPclient) SikGenProof() (*SikProof, error) {
	resp, code, err := c.Request(HTTPGET, nil, "sik", "proof", c.account.AddressString())
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	sikProof := &api.SikProof{}
	if err := json.Unmarshal(resp, sikProof); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	sp := SikProof{
		Proof:    sikProof.Proof,
		Root:     sikProof.Root,
		Siblings: sikProof.Siblings,
	}
	return &sp, nil
}
