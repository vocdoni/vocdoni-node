package config

// These consts are defaults used in VochainCfg
const (
	DefaultMinerTargetBlockTimeSeconds = 10
	DefaultCometBFTPath                = "cometbft"
	DefaultGenesisPath                 = DefaultCometBFTPath + "/config/genesis.json"
)

// DefaultSeedNodes is a map indexed by network name
var DefaultSeedNodes = map[string][]string{
	// testsuite test network
	"test": {
		"3c3765494e758ae7baccb1f5b0661755302ddc47@seed:26656",
	},
	// Development network
	"dev": {
		"7440a5b086e16620ce7b13198479016aa2b07988@seed.dev.vocdoni.net:26656",
	},

	// Staging network
	"stage": {
		"588133b8309363a2a852e853424251cd6e8c5330@seed.stg.vocdoni.net:26656",
	},

	// LTS production network
	"lts": {
		"32acbdcda649fbcd35775f1dd8653206d940eee4@seed1.lts.vocdoni.net:26656",
		"02bfac9bd98bf25429d12edc50552cca5e975080@seed2.lts.vocdoni.net:26656",
	},
}
