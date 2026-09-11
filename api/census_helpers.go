package api

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

// decodeCensusType decodes the given census type string to a valid
// models.Census_Type value, by default models.Census_UNKNOWN. This function
// also returns a boolean indicating whether the current census is indexed or
// not.
func decodeCensusType(t string) models.Census_Type {
	switch t {
	case CensusTypeZKWeighted:
		return models.Census_ARBO_POSEIDON
	case CensusTypeWeighted:
		return models.Census_ARBO_BLAKE2B
	}
	return models.Census_UNKNOWN
}

// encodeCensusType returns the string version of the given models.Census_Type, by
// default CensusTypeUnknown. Note: after the Farcaster removal (#1458), historical
// elections carrying a retired census origin serialise as CensusTypeUnknown, and
// api/elections.go's models.CensusOrigin_name[proc.CensusOrigin] lookup returns
// "" for enum integers that no longer exist in dvote-protobuf v1.16+.
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
		return nil, ErrCensusIDLengthInvalid.Withf("(%d != %d)", len(censusID), censusIDsize*2)
	}
	return hex.DecodeString(censusID)
}

func censusKeyParse(key string) ([]byte, error) {
	key = util.TrimHex(key)
	return hex.DecodeString(key)
}

func addressParse(key string) common.Address {
	return common.HexToAddress(util.TrimHex(key))
}
