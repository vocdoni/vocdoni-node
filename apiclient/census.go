package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
)

func (c *HTTPclient) NewCensus(censusType string) (types.HexBytes, error) {
	// create a new census
	resp, code, err := c.Request("GET", nil, "census", "create", censusType)
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

func (c *HTTPclient) CensusAddVoter(censusID, voterPubKey types.HexBytes, weight uint64) error {
	var resp []byte
	var code int
	var err error
	if weight == 0 {
		resp, code, err = c.Request("GET", nil, "census", censusID.String(), "add", voterPubKey.String())
	}
	if weight > 0 {
		resp, code, err = c.Request("GET", nil, "census", censusID.String(), "add", voterPubKey.String(), fmt.Sprintf("%d", weight))
	}
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	return nil
}

func (c *HTTPclient) CensusPublish(censusID types.HexBytes) (types.HexBytes, error) {
	resp, code, err := c.Request("GET", nil, "census", censusID.String(), "publish")
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
