package config

// ForksCfg holds the heights at which soft forks activate on a chain. A zero
// height means the fork is active from genesis.
type ForksCfg struct {
	// MetadataFork activates the process metadata rules (issue #1479): the
	// SET_PROCESS_METADATA transaction, the metadata URI/hash checks on
	// NewProcessTx and the metadata hash attestation on votes. Before it, the
	// chain must behave exactly as binaries without these rules.
	MetadataFork uint32
	// LegacyCSPFork rejects NewProcessTx with CensusOrigin OFF_CHAIN_CA, whose
	// salt derivation is broken (issue #1424). Before it, the legacy origin is
	// accepted with a deprecation warning.
	LegacyCSPFork uint32
}

// Forks maps chainIDs to their soft fork heights. Chains not listed activate
// every fork from genesis.
var Forks = map[string]*ForksCfg{
	"vocdoni/DEV/36": {
		MetadataFork: 40_793,
	},
	"vocdoni/LTS/1.3": {
		// ~2026-10-08 07:00 UTC
		MetadataFork: 8_705_150,
		// TODO: set the final activation height before releasing to LTS.
		LegacyCSPFork: 9_000_000,
	},
}

// ForksForChainID returns the ForksCfg of chainID, if found, or an empty
// ForksCfg (every fork active from genesis) otherwise.
func ForksForChainID(chainID string) *ForksCfg {
	if cfg, found := Forks[chainID]; found {
		return cfg
	}
	return &ForksCfg{}
}
