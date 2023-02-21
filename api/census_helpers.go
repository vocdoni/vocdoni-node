package api

import (
	"encoding/hex"
	"fmt"

	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

// decodeCensusType decodes the given census type string to a valid
// models.Census_Type value, by default models.Census_UNKNOWN. This function
// also returns a boolean indicating whether the current census is indexed or
// not.
func decodeCensusType(t string) (models.Census_Type, bool) {
	switch t {
	case CensusTypeZK:
		return models.Census_ARBO_POSEIDON, true
	case CensusTypeZKWeighted:
		return models.Census_ARBO_POSEIDON, false
	case CensusTypeWeighted:
		return models.Census_ARBO_BLAKE2B, false
	}
	return models.Census_UNKNOWN, false
}

// encodeCensusType returns the string version of the given models.Census_Type, by
// default CensusTypeUnknown.
func encodeCensusType(t models.Census_Type) string {
	switch t {
	case models.Census_ARBO_POSEIDON:
		return CensusTypeZKWeighted
	case models.Census_ARBO_BLAKE2B:
		return CensusTypeWeighted
	case models.Census_CA:
		return CensusTypeCSP
	}

	return CensusTypeUnknown
}

func censusIDparse(censusID string) ([]byte, error) {
	censusID = util.TrimHex(censusID)
	if len(censusID) != censusIDsize*2 {
		return nil, fmt.Errorf("%w (%d != %d)", ErrCensusIDLengthInvalid, len(censusID), censusIDsize*2)

	}
	return hex.DecodeString(censusID)
}

func censusKeyParse(key string) ([]byte, error) {
	key = util.TrimHex(key)
	return hex.DecodeString(key)
}
