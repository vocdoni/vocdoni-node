package api

import (
	"encoding/hex"
	"fmt"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func censusType(t string) (models.Census_Type, bool) {
	switch t {
	case CensusTypeZK:
		return models.Census_ARBO_POSEIDON, true
	case CensusTypeWeighted:
		return models.Census_ARBO_BLAKE2B, false
	}
	return models.Census_UNKNOWN, false
}

func censusIDparse(censusID string) ([]byte, error) {
	censusID = util.TrimHex(censusID)
	if len(censusID) != censusIDsize*2 {
		return nil, fmt.Errorf("invalid censusID format")
	}
	return hex.DecodeString(censusID)
}

func censusKeyParse(key string) ([]byte, error) {
	key = util.TrimHex(key)
	return hex.DecodeString(key)
}

func censusWeightParse(w string) (*types.BigInt, error) {
	weight, err := new(types.BigInt).SetString(w)
	if err != nil {
		return nil, err
	}
	return weight, nil
}
