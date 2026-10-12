package state

import "go.vocdoni.io/proto/build/go/models"

// CensusProperties contains the properties of the different census origins.
type CensusProperties struct {
	Name              string
	AllowCensusUpdate bool
	NeedsIndexSlot    bool
	NeedsURI          bool
	WeightedSupport   bool
}

// CensusOrigins is a map of the different census origins and their properties.
// Only CSP-based origins are supported.
var CensusOrigins = map[models.CensusOrigin]CensusProperties{
	models.CensusOrigin_OFF_CHAIN_CA: {
		Name: "ca", WeightedSupport: true,
		NeedsURI: true, AllowCensusUpdate: true,
	},
	models.CensusOrigin_OFF_CHAIN_CA_V2: {
		Name: "ca_v2", WeightedSupport: true,
		NeedsURI: true, AllowCensusUpdate: true,
	},
}
