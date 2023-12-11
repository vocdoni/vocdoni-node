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
		VoceremonyForkBlock: 249500, // estimated 2023-12-11T10:38:31.862340282Z
	},
	"vocdoni/LTS/1.2": {
		VoceremonyForkBlock: 393000, // estimated 2023-12-11T11:51:47.046130989Z
	},
}

// ForksForChainID returns the ForksCfg of chainID, if found, or an empty ForksCfg otherwise
func ForksForChainID(chainID string) *ForksCfg {
	if cfg, found := Forks[chainID]; found {
		return cfg
	}
	return &ForksCfg{}
}
