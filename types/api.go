package types

// MessageRequest holds a decoded request but does not decode the body
type RequestMessage struct {
	ID string `json:"id"`
	//	Request   map[string]interface{} `json:"request"`
	Request   MetaRequest `json:"request"`
	Signature string      `json:"signature"`
}

// MetaRequest contains all of the possible request fields.
// Fields must be in alphabetical order
type MetaRequest struct {
	CensusID       string   `json:"censusId,omitempty"`
	CensusURI      string   `json:"censusUri,omitempty"`
	ClaimData      string   `json:"claimData,omitempty"`
	ClaimsData     []string `json:"claimsData,omitempty"`
	Content        string   `json:"content,omitempty"`
	From           int64    `json:"from,omitempty"`
	ListSize       int64    `json:"listSize,omitempty"`
	Method         string   `json:"method"`
	Name           string   `json:"name,omitempty"`
	Nullifier      string   `json:"nullifier,omitempty"`
	Payload        *VoteTx  `json:"payload,omitempty"`
	ProcessID      string   `json:"processId,omitempty"`
	PubKeys        []string `json:"pubKeys,omitempty"`
	RootHash       string   `json:"rootHash,omitempty"`
	Signature      string   `json:"signature,omitempty"`
	Timestamp      int32    `json:"timestamp"`
	Type           string   `json:"type,omitempty"`
	URI            string   `json:"uri,omitempty"`
	VotePublicKeys []string `json:"encryptionPublicKeys,omitempty"`
}

// ResponseMessage wraps an api response
type ResponseMessage struct {
	ID        string       `json:"id"`
	Response  MetaResponse `json:"response"`
	Signature string       `json:"signature"`
}

// ErrorMessage wraps an api error
type ErrorMessage struct {
	ID        string       `json:"id"`
	Error     MetaResponse `json:"error"`
	Signature string       `json:"signature"`
}

// MetaResponse contains all of the possible request fields.
// Fields must be in alphabetical order
type MetaResponse struct {
	CensusID      string    `json:"censusId,omitempty"`
	ClaimsData    []string  `json:"claimsData,omitempty"`
	Content       string    `json:"content,omitempty"`
	Error         string    `json:"error,omitempty"`
	Files         []byte    `json:"files,omitempty"`
	Height        *int64    `json:"height,omitempty"`
	InvalidClaims []int     `json:"invalidClaims,omitempty"`
	Message       string    `json:"message,omitempty"`
	Nullifier     string    `json:"nullifier,omitempty"`
	Nullifiers    *[]string `json:"nullifiers,omitempty"`
	Ok            *bool     `json:"ok,omitempty"`
	Payload       string    `json:"payload,omitempty"`
	Registered    *bool     `json:"registered,omitempty"`
	Request       string    `json:"request"`
	Root          string    `json:"root,omitempty"`
	Siblings      string    `json:"siblings,omitempty"`
	Size          int64     `json:"size,omitempty"`
	Timestamp     int32     `json:"timestamp"`
	URI           string    `json:"uri,omitempty"`
	ValidProof    bool      `json:"validProof,omitempty"`
}

type CensusDump struct {
	RootHash   string   `json:"rootHash"`
	ClaimsData []string `json:"claimsData"`
}
