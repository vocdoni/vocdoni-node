package types

import "time"

const (
	// The mode defines the behaviour of the vocdoninode

	// ModeMiner starts vocdoninode as a miner.
	ModeMiner = "miner"
	// ModeSeed starts vocdoninode as a seed node.
	ModeSeed = "seed"
	// ModeGateway starts the vocdoninode as a gateway.
	ModeGateway = "gateway"
	// ModeCensus starts the vocdoninode as a census only service.
	ModeCensus = "census"

	// ProcessIDsize is the size of a process id.
	ProcessIDsize = 32

	// EthereumAddressSize is the size of an ethereum address.
	EthereumAddressSize = 20

	// EntityIDsize is the size of an entity id (ethereum address).
	EntityIDsize = EthereumAddressSize

	// DefaultBlockTime is the default block time in seconds.
	DefaultBlockTime = 10 * time.Second

	// KeyKeeperMaxKeyIndex is the maxim number of allowed encryption keys.
	KeyKeeperMaxKeyIndex = 16

	// ProcessesContractMaxEnvelopeType represents the max value that a uint8 can have
	// with the current smart contract bitmask describing the supported envelope types.
	ProcessesContractMaxEnvelopeType = 31

	// MaxURLLength is the maximum length of a URL string used in the protocol.
	MaxURLLength = 2083
)
