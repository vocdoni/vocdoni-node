package types

func Bool(b bool) *bool { return &b }

// These exported variables should be treated as constants, to be used in API
// responses which require *bool fields.
var (
	False = Bool(false)
	True  = Bool(true)
)

const (
	// ScrutinizerProcessPrefix is the prefix for the levelDB process keys
	ScrutinizerProcessPrefix = "p_"
	// ScrutinizerEntityPrefix is the prefix for the levelDB entity keys
	ScrutinizerEntityPrefix = "e_"
	// PetitionSign contains the string that needs to match with the received vote type for petition-sign
	PetitionSign = "petition-sign"
	// PollVote contains the string that needs to match with the received vote type for poll-vote
	PollVote = "poll-vote"
	// EncryptedPoll contains the string that needs to match with the received vote type for encrypted-poll
	EncryptedPoll = "encrypted-poll"
	// SnarkVote contains the string that needs to match with the received vote type for snark-vote
	SnarkVote = "snark-vote"
)
