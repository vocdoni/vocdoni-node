package types

import (
	"time"
)

func Bool(b bool) *bool { return &b }

// These exported variables should be treated as constants, to be used in API
// responses which require *bool fields.
var (
	False = Bool(false)
	True  = Bool(true)
)

const (
	// All

	// The mode defines the behaviour of the vocdoninode

	// ModeMiner starts vocdoninode as a miner
	ModeMiner = "miner"
	// ModeSeed starts vocdoninode as a seed node
	ModeSeed = "seed"
	// ModeGateway starts the vocdoninode as a gateway
	ModeGateway = "gateway"

	// ProcessIDsize is the size of a process id
	ProcessIDsize = 32
	// EthereumAddressSize is the size of an ethereum address
	EthereumAddressSize = 20

	// EntityIDsizeV2 legacy: in the past we used hash(addr)
	// this is a temporal work around to support both
	EntityIDsize = 20
	// KeyIndexSeparator is the default char used to split keys
	KeyIndexSeparator = ":"
	// EthereumConfirmationsThreshold is the minimum amout of blocks
	// that should pass before considering a tx final
	EthereumConfirmationsThreshold = 6

	// ENS Domains

	// ENTITY RESOLVER
	// EntityResolverDomain is the default entity resolver ENS domain
	EntityResolverDomain = "entities.voc.eth"
	// EntityResolverStageDomain is the default entity resolver ENS domain
	EntityResolverStageDomain = "entities.stg.voc.eth"
	// EntityResolverDevelopmentDomain is the default entity resolver ENS domain
	EntityResolverDevelopmentDomain = "entities.dev.voc.eth"

	// PROCESSES
	// ProcessesDomain
	ProcessesDomain = "processes.voc.eth"
	// ProcessesStageDomain
	ProcessesStageDomain = "processes.stg.voc.eth"
	// ProcessesDevelopmentDomain
	ProcessesDevelopmentDomain = "processes.dev.voc.eth"

	// NAMESPACES
	// NamespacesDomain
	NamespacesDomain = "namespaces.voc.eth"
	// NamespacesStageDomain
	NamespacesStageDomain = "namespaces.stg.voc.eth"
	// NamespacesDevelopmentDomain
	NamespacesDevelopmentDomain = "namespaces.dev.voc.eth"

	// ERC20 PROOFS
	// ERC20ProofsDomain
	ERC20ProofsDomain = "erc20.proofs.voc.eth"
	// ERC20ProofsStageDomain
	ERC20ProofsStageDomain = "erc20.proofs.stg.voc.eth"
	// ERC20ProofsDevelopmentDomain
	ERC20ProofsDevelopmentDomain = "erc20.proofs.dev.voc.eth"

	// GENESIS
	// GenesisDomain
	GenesisDomain = "genesis.voc.eth"
	// GenesisStageDomain
	GenesisStageDomain = "genesis.stg.voc.eth"
	// GenesisDevelopmentDomain
	GenesisDevelopmentDomain = "genesis.dev.voc.eth"

	// RESULTS
	// ResultsDomain
	ResultsDomain = "results.voc.eth"
	// ResultsStageDomain
	ResultsStageDomain = "results.stg.voc.eth"
	// ResultsDevelopmentDomain
	ResultsDevelopmentDomain = "results.dev.voc.eth"

	// EntityMetaKey is the key of an ENS text record for the entity metadata
	EntityMetaKey = "vnd.vocdoni.meta"

	// EthereumReadTimeout is the max amount of time for reading anything on
	// the Ethereum network to wait until canceling it's context
	EthereumReadTimeout = 1 * time.Minute
	// EthereumWriteTimeout is the max amount of time for writing anything on
	// the Ethereum network to wait until canceling it's context
	EthereumWriteTimeout = 1 * time.Minute
	// EthereumDialMaxRetry is the max number of attempts an ethereum client will
	// make in order to dial to an endpoint before considering the endpoint unreachable
	EthereumDialMaxRetry = 10

	// Indexer

	// IndexerLiveProcessPrefix is used for sotring temporary results on live
	IndexerLiveProcessPrefix = byte(0x21)
	// IndexerEntityPrefix is the prefix for the storage entity keys
	IndexerEntityPrefix = byte(0x22)
	// IndexerResultsPrefix is the prefix of the storage results summary keys
	IndexerResultsPrefix = byte(0x24)
	// IndexerProcessEndingPrefix is the prefix for keep track of the processes ending
	// on a specific block
	IndexerProcessEndingPrefix = byte(0x25)

	// Vochain

	// PetitionSign contains the string that needs to match with the received vote type
	// for petition-sign
	PetitionSign = "petition-sign"
	// PollVote contains the string that needs to match with the received vote type for poll-vote
	PollVote = "poll-vote"
	// EncryptedPoll contains the string that needs to match with the received vote type
	// for encrypted-poll
	EncryptedPoll = "encrypted-poll"
	// SnarkVote contains the string that needs to match with the received vote type for snark-vote
	SnarkVote = "snark-vote"

	// KeyKeeper

	// KeyKeeperMaxKeyIndex is the maxim number of allowed encryption keys
	KeyKeeperMaxKeyIndex = 16

	// List of transition names

	TxVote              = "vote"
	TxNewProcess        = "newProcess"
	TxCancelProcess     = "cancelProcess" // legacy
	TxSetProcess        = "setProcess"
	TxAddValidator      = "addValidator"
	TxRemoveValidator   = "removeValidator"
	TxAddProcessKeys    = "addProcessKeys"
	TxRevealProcessKeys = "revealProcessKeys"

	// ProcessesContractMaxProcessMode represents the max value that a uint8 can have
	// with the current smart contract bitmask describing the supported process mode
	ProcessesContractMaxProcessMode = 31
	// ProcessesContractMaxEnvelopeType represents the max value that a uint8 can have
	// with the current smart contract bitmask describing the supported envelope types
	ProcessesContractMaxEnvelopeType = 31

	// TODO: @jordipainan this values are tricky

	// ProcessesContractMinBlockCount represents the minimum number of vochain blocks
	// that a process should last
	ProcessesContractMinBlockCount = 2

	// ProcessesParamsSignatureSize represents the size of a signature on ethereum
	ProcessesParamsSignatureSize = 32

	VochainWsReadLimit = 20 << 20 // tendermint requires 20 MiB minimum
	Web3WsReadLimit    = 5 << 20  // go-ethereum accepts maximum 5 MiB

	MaxURLLength = 2083
)
