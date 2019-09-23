package vochain

// ________________________ VOTE ________________________

// Vote represents a single vote either for snark votes or polls
type Vote struct {
	// ProcessId contains the vote itself
	ProcessID string `json:"processId"`
	// Proof contains the prove indicating that the user is in the census of the process
	Proof string `json:"proof"`
	// Nullifier is the hash of the private key
	Nullifier string `json:"nullifier"`
	// VotePackage base64 encoded vote content
	VotePackage string `json:"votePackage"`
	// Nonce unique number per vote attempt, so that replay attacks can't reuse this payload
	Nonce string `json:"nonce"`
	// Signature sign( JSON.stringify( { nonce, processId, proof, 'vote-package' } ), privateKey )
	Signature string `json:"signature"`
}

// NewVote returns a new Vote instance
func NewVote() *Vote {
	return &Vote{}
}
