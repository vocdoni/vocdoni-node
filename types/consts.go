package types

func Bool(b bool) *bool { return &b }

// These exported variables should be treated as constants, to be used in API
// responses which require *bool fields.
var (
	False = Bool(false)
	True  = Bool(true)
)

const (
	ProcessIDsize = 32
	// size of eth addr
	EntityIDsize = 20
	// legacy: in the past we used hash(addr)
	// this is a temporal work around to support both
	EntityIDsizeV2    = 32
	VoteNullifierSize = 32
	// ScrutinizerProcessPrefix is the prefix for the storage process keys
	ScrutinizerLiveProcessPrefix = "p_"
	// ScrutinizerEntityPrefix is the prefix for the storage entity keys
	ScrutinizerEntityPrefix = "e_"
	// ScrutinizerResultsPrefix is the prefix of the storage results summary keys
	ScrutinizerResultsPrefix = "r_"
	// ScrutinizerProcessEndingPrefix is the prefix for keep track of the processes ending on a specific block
	ScrutinizerProcessEndingPrefix = "s_"
	// PetitionSign contains the string that needs to match with the received vote type for petition-sign
	PetitionSign = "petition-sign"
	// PollVote contains the string that needs to match with the received vote type for poll-vote
	PollVote = "poll-vote"
	// EncryptedPoll contains the string that needs to match with the received vote type for encrypted-poll
	EncryptedPoll = "encrypted-poll"
	// SnarkVote contains the string that needs to match with the received vote type for snark-vote
	SnarkVote = "snark-vote"
	// AdminTxAddProcessKeys contains the string that needs to match with the received adminTx.Type for adding process keys
	AdminTxAddProcessKeys = "addProcessKeys"
	// MaxKeyIndex is the maxim number of allowed Encryption or Commitment keys
	MaxKeyIndex = 16
)
