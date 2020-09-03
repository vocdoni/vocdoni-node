package types

func Bool(b bool) *bool { return &b }

// These exported variables should be treated as constants, to be used in API
// responses which require *bool fields.
var (
	False = Bool(false)
	True  = Bool(true)
)

const (
	// All

	ProcessIDsize = 32
	// size of eth addr
	EntityIDsize = 20
	// legacy: in the past we used hash(addr)
	// this is a temporal work around to support both
	EntityIDsizeV2                 = 32
	VoteNullifierSize              = 32
	KeyIndexSeparator              = ":"
	EthereumConfirmationsThreshold = 6
	EntityResolverDomain           = "entity-resolver.vocdoni.eth"
	EntityMetaKey                  = "vnd.vocdoni.meta"

	// Scrutinizer

	// ScrutinizerLiveProcessPrefix is used for sotring temporary results on live
	ScrutinizerLiveProcessPrefix = "p_"
	// ScrutinizerEntityPrefix is the prefix for the storage entity keys
	ScrutinizerEntityPrefix = "e_"
	// ScrutinizerEntityProcessSeparator char for spliting process ID's in the scrutinizer entities
	ScrutinizerEntityProcessSeparator = "_"
	// ScrutinizerResultsPrefix is the prefix of the storage results summary keys
	ScrutinizerResultsPrefix = "r_"
	// ScrutinizerProcessEndingPrefix is the prefix for keep track of the processes ending on a specific block
	ScrutinizerProcessEndingPrefix = "s_"

	// Vochain

	// PetitionSign contains the string that needs to match with the received vote type for petition-sign
	PetitionSign = "petition-sign"
	// PollVote contains the string that needs to match with the received vote type for poll-vote
	PollVote = "poll-vote"
	// EncryptedPoll contains the string that needs to match with the received vote type for encrypted-poll
	EncryptedPoll = "encrypted-poll"
	// SnarkVote contains the string that needs to match with the received vote type for snark-vote
	SnarkVote = "snark-vote"

	// List of transation names
	TxVote              = "vote"
	TxNewProcess        = "newProcess"
	TxCancelProcess     = "cancelProcess"
	TxAddValidator      = "addValidator"
	TxRemoveValidator   = "removeValidator"
	TxAddOracle         = "addOracle"
	TxRemoveOracle      = "removeOracle"
	TxAddProcessKeys    = "addProcessKeys"
	TxRevealProcessKeys = "revealProcessKeys"

	// MaxKeyIndex is the maxim number of allowed Encryption or Commitment keys
	MaxKeyIndex = 16
)
