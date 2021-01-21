package types

import "time"

func Bool(b bool) *bool { return &b }

// These exported variables should be treated as constants, to be used in API
// responses which require *bool fields.
var (
	False = Bool(false)
	True  = Bool(true)
)

const (
	// All

	// The mode defines the behaviour of the dvotenode

	// ModeOracle start dvotenode as an oracle
	ModeOracle = "oracle"
	// ModeMiner start dvotenode as a miner
	ModeMiner = "miner"
	// ModeGateway start the dvotenode as a gateway
	ModeGateway = "gateway"
	// ModeWeb3 start the dvotenode as a web3 gateway
	ModeWeb3 = "web3"

	// ProcessIDsize is the size of a process id
	ProcessIDsize = 32
	// EntityIDsize is the size of an entity id which is the same of an ethereum address
	EntityIDsize = 20

	// EntityIDsizeV2 legacy: in the past we used hash(addr)
	// this is a temporal work around to support both
	EntityIDsizeV2 = 32
	// VoteNullifierSize is the size of a vote nullifier
	VoteNullifierSize = 32
	// KeyIndexSeparator is the default char used to split keys
	KeyIndexSeparator = ":"
	// EthereumConfirmationsThreshold is the minimum amout of blocks
	// that should pass before considering a tx final
	EthereumConfirmationsThreshold = 6
	// EntityResolverDomain is the default entity resolver ENS domain
	EntityResolverDomain = "entity-resolver.vocdoni.eth"
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

	// Scrutinizer

	// ScrutinizerLiveProcessPrefix is used for sotring temporary results on live
	ScrutinizerLiveProcessPrefix = byte(0x21)
	// ScrutinizerEntityPrefix is the prefix for the storage entity keys
	ScrutinizerEntityPrefix = byte(0x22)
	// ScrutinizerResultsPrefix is the prefix of the storage results summary keys
	ScrutinizerResultsPrefix = byte(0x24)
	// ScrutinizerProcessEndingPrefix is the prefix for keep track of the processes ending on a specific block
	ScrutinizerProcessEndingPrefix = byte(0x25)

	// Vochain

	// PetitionSign contains the string that needs to match with the received vote type for petition-sign
	PetitionSign = "petition-sign"
	// PollVote contains the string that needs to match with the received vote type for poll-vote
	PollVote = "poll-vote"
	// EncryptedPoll contains the string that needs to match with the received vote type for encrypted-poll
	EncryptedPoll = "encrypted-poll"
	// SnarkVote contains the string that needs to match with the received vote type for snark-vote
	SnarkVote = "snark-vote"

	// KeyKeeper

	// KeyKeeperMaxKeyIndex is the maxim number of allowed Encryption or Commitment keys
	KeyKeeperMaxKeyIndex = 16

	// List of transation names

	TxVote              = "vote"
	TxNewProcess        = "newProcess"
	TxCancelProcess     = "cancelProcess" // legacy
	TxAddValidator      = "addValidator"
	TxRemoveValidator   = "removeValidator"
	TxAddOracle         = "addOracle"
	TxRemoveOracle      = "removeOracle"
	TxAddProcessKeys    = "addProcessKeys"
	TxRevealProcessKeys = "revealProcessKeys"

	// ProcessesContractMaxProcessMode represents the max value that a uint8 can have
	// with the current smart contract bitmask describing the supported process mode
	ProcessesContractMaxProcessMode = 15
	// ProcessesContractMaxEnvelopeType represents the max value that a uint8 can have
	// with the current smart contract bitmask describing the supported envelope types
	ProcessesContractMaxEnvelopeType = 15

	// TODO: @jordipainan this values are tricky

	// ProcessesContractMinStartBlock represents the minimum vochain block number
	// can be started where a process
	ProcessesContractMinStartBlock = 0
	// ProcessesContractMinBlockCount represents the minimum number of vochain blocks
	// that a process should last
	ProcessesContractMinBlockCount = 0

	// ProcessesParamsSignatureSize represents the size of a signature on ethereum
	ProcessesParamsSignatureSize = 32
)
