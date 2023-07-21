package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/apirest"
)

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
func (c *HTTPclient) SikGenProof() (*CensusProof, error) {
	resp, code, err := c.Request(HTTPGET, nil, "sik", "proof", c.account.AddressString())
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	sikData := &api.Census{}
	if err := json.Unmarshal(resp, sikData); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	cp := CensusProof{
		Root:      sikData.CensusRoot,
		Proof:     sikData.CensusProof,
		LeafValue: sikData.Value,
		Siblings:  sikData.CensusSiblings,
	}
	return &cp, nil
}
