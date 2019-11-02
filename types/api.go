package types

/*MessageRequest holds a decoded request but does not decode the body*/
type RequestMessage struct {
	ID string `json:"id"`
	//	Request   map[string]interface{} `json:"request"`
	Request   MetaRequest `json:"request"`
	Signature string      `json:"signature"`
}

/*MetaRequest contains all of the possible request fields.
Fields must be in alphabetical order */
type MetaRequest struct {
	CensusID   string          `json:"censusId,omitempty"`
	CensusURI  string          `json:"censusUri,omitempty"`
	ClaimData  string          `json:"claimData,omitempty"`
	ClaimsData []string        `json:"claimsData,omitempty"`
	Content    string          `json:"content,omitempty"`
	From       int64           `json:"from,omitempty"`
	ListSize   int64           `json:"listSize,omitempty"`
	Method     string          `json:"method"`
	Name       string          `json:"name,omitempty"`
	Nullifier  string          `json:"nullifier,omitempty"`
	Payload    EnvelopePayload `json:"payload,omitempty"`
	ProcessId  string          `json:"processId,omitempty"`
	PubKeys    []string        `json:"encryptionPublicKeys,omitempty"`
	RootHash   string          `json:"rootHash,omitempty"`
	Timestamp  int32           `json:"timestamp"`
	Type       string          `json:"type,omitempty"`
	URI        string          `json:"uri,omitempty"`
	Signature  string          `json:"signature,omitempty"`
}

type EnvelopePayload struct {
	VotePackage string `json:"vote-package"`
	Nullifier   string `json:"nullifier,omitempty"`
	Nonce       string `json:"nonce,omitempty"`
	Proof       string `json:"proof,omitempty"`
	Signature   string `json:"signature,omitempty"`
	ProcessId   string `json:"processId,omitempty"`
}

//ResponseMessage wraps an api response
type ResponseMessage struct {
	ID        string       `json:"id"`
	Response  MetaResponse `json:"response"`
	Signature string       `json:"signature"`
}

//ErrorMessage wraps an api error
type ErrorMessage struct {
	ID        string       `json:"id"`
	Error     MetaResponse `json:"error"`
	Signature string       `json:"signature"`
}

/*MetaResponse contains all of the possible request fields.
Fields must be in alphabetical order */
type MetaResponse struct {
	CensusID      string   `json:"censusId,omitempty"`
	ClaimsData    []string `json:"claimsData,omitempty"`
	Content       string   `json:"content,omitempty"`
	Error         string   `json:"error,omitempty"`
	Files         []byte   `json:"files,omitempty"`
	Height        int64    `json:"height,omitempty"`
	InvalidClaims []int    `json:"invalidClaims,omitempty"`
	Message       string   `json:"message,omitempty"`
	Nullifiers    []string `json:"nullifiers,omitempty"`
	Ok            bool     `json:"ok,omitempty"`
	Payload       string   `json:"payload,omitempty"`
	Registered    string   `json:"registered,omitempty"`
	Request       string   `json:"request"`
	Root          string   `json:"root,omitempty"`
	Siblings      string   `json:"siblings,omitempty"`
	Size          int64    `json:"size,omitempty"`
	Timestamp     int32    `json:"timestamp"`
	URI           string   `json:"uri,omitempty"`
	ValidProof    bool     `json:"validProof,omitempty"`
}

type CensusDump struct {
	RootHash   string   `json:"rootHash"`
	ClaimsData []string `json:"claimsData"`
}
