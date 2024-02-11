package state

import "go.vocdoni.io/proto/build/go/models"

// CensusProperties contains the properties of the different census origins.
type CensusProperties struct {
	Name              string
	AllowCensusUpdate bool
	NeedsDownload     bool
	NeedsIndexSlot    bool
	NeedsURI          bool
	WeightedSupport   bool
}

// CensusOrigins is a map of the different census origins and their properties.
var CensusOrigins = map[models.CensusOrigin]CensusProperties{
	models.CensusOrigin_OFF_CHAIN_TREE: {
		Name:          "offchain tree",
		NeedsDownload: true, NeedsURI: true, AllowCensusUpdate: true,
	},
	models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED: {
		Name: "offchain weighted tree", NeedsDownload: true, NeedsURI: true,
		WeightedSupport: true, AllowCensusUpdate: true,
	},
	models.CensusOrigin_ERC20: {
		Name: "erc20", NeedsDownload: true,
		WeightedSupport: true, NeedsIndexSlot: true,
	},
	models.CensusOrigin_OFF_CHAIN_CA: {
		Name: "ca", WeightedSupport: true,
		NeedsURI: true, AllowCensusUpdate: true,
	},
	models.CensusOrigin_FARCASTER_FRAME: {
		Name: "farcaster", NeedsDownload: true,
		NeedsURI: true, AllowCensusUpdate: true, WeightedSupport: true,
	},
}
