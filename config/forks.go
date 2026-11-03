package config

import "math"

// NotScheduled is the activation height of a fork whose deploy date on a chain
// is not decided yet. No chain reaches it, so the fork stays inactive there;
// zero instead would activate it from genesis and break the replay of the
// existing blocks.
const NotScheduled = math.MaxUint32

// ForksCfg holds the heights at which soft forks activate on a chain. A zero
// height means the fork is active from genesis.
type ForksCfg struct {
	// MetadataFork activates the process metadata rules (issue #1479): the
	// SET_PROCESS_METADATA transaction, the metadata URI/hash checks on
	// NewProcessTx and the metadata hash attestation on votes. Before it, the
	// chain must behave exactly as binaries without these rules.
	MetadataFork uint32
	// ParentFork activates metadata-only processes (no voteOptions, envelope
	// type nor census, so they take no votes and have no results), the
	// parentProcessId link from a process to one of them, and the attestation
	// of the parent metadata hash on votes. Before it, the chain must behave
	// exactly as binaries without these rules.
	ParentFork uint32
}

// Forks maps chainIDs to their soft fork heights. Chains not listed activate
// every fork from genesis.
var Forks = map[string]*ForksCfg{
	"vocdoni/DEV/36": {
		MetadataFork: 40_793,
		ParentFork:   NotScheduled,
	},
	"vocdoni/LTS/1.3": {
		// ~2026-10-08 07:00 UTC
		MetadataFork: 8_705_150,
		ParentFork:   NotScheduled,
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
