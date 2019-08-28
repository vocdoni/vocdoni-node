package vochain

// ________________________ VOTE ________________________

// Vote represents a single vote
type Vote struct {
	// Payload contains the vote itself
	Payload string `json:"payload"`
	// CensusProof contains the prove indicating that the user is in the census of the process
	CensusProof string `json:"censusproof"`
	// Nullifier avoids double voting
	Nullifier string `json:"nullifier"`
}

// NewVote returns a new Vote instance
func NewVote() *Vote {
	return &Vote{}
}
