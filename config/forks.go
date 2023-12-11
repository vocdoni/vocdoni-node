package config

// ForksCfg allows applying softforks at specified heights
type ForksCfg struct {
	VoceremonyForkBlock uint32
}

// Forks is a map of chainIDs
var Forks = map[string]*ForksCfg{
	"vocdoni/DEV/29": {
		VoceremonyForkBlock: 217200, // estimated 2023-12-05T11:33:31.426638381Z
	},
	"vocdoni/STAGE/9": {
		VoceremonyForkBlock: 250000, // estimated 2023-12-11T12:09:00.917676214Z
	},
	"vocdoni/LTS/1.2": {
		VoceremonyForkBlock: 400200, // estimated 2023-12-12T09:09:31.511245938Z
	},
}

// ForksForChainID returns the ForksCfg of chainID, if found, or an empty ForksCfg otherwise
func ForksForChainID(chainID string) *ForksCfg {
	if cfg, found := Forks[chainID]; found {
		return cfg
	}
	return &ForksCfg{}
}
