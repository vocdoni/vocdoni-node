package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/apirest"
)

type SIKRoots []string

// ValidSIK checks if the current client account has a valid SIK registered in
// the vochain
func (c *HTTPclient) ValidSIK() (bool, error) {
	resp, code, err := c.Request(HTTPGET, nil, "siks", c.account.AddressString())
	if err != nil {
		return false, err
	}
	if code != apirest.HTTPstatusOK {
		return false, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	return true, nil
}

// ValidSIKRoots returns the currently valid roots of SIK merkle tree from the
// API.
func (c *HTTPclient) ValidSIKRoots() (SIKRoots, error) {
	resp, code, err := c.Request(HTTPGET, nil, "siks", "roots")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	sp := struct {
		SIKRoots []string `json:"sikroots"`
	}{}
	if err := json.Unmarshal(resp, &sp); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return sp.SIKRoots, nil
}

// GenSIKProof generates a proof for the voter address in the sik merkle tree.
func (c *HTTPclient) GenSIKProof() (*CensusProof, error) {
	resp, code, err := c.Request(HTTPGET, nil, "siks", "proof", c.account.AddressString())
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
