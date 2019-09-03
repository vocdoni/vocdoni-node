package types

/*MessageRequest holds a decoded request but does not decode the body*/
type MessageRequest struct {
	ID string `json:"id"`
	//	Request   map[string]interface{} `json:"request"`
	Request   MetaRequest `json:"request"`
	Signature string      `json:"signature"`
}

//MetaRequest holds all possible request methods ordered by field name. It's used by the router to check the signature
type MetaRequest struct {
	CensusID   string   `json:"censusId,omitempty"`
	CensusURI  string   `json:"censusUri,omitempty"`
	ClaimData  string   `json:"claimData,omitempty"`
	ClaimsData []string `json:"claimsData,omitempty"`
	Content    string   `json:"content,omitempty"`
	Method     string   `json:"method"`
	Name       string   `json:"name,omitempty"`
	ProofData  string   `json:"proofData,omitempty"`
	PubKeys    []string `json:"pubKeys,omitempty"`
	RootHash   string   `json:"rootHash,omitempty"`
	Timestamp  int32    `json:"timestamp"`
	Type       string   `json:"type"`
	URI        string   `json:"uri,omitempty"`
}

/* the following structs hold content decoded from File API JSON objects */

//FetchResponse holds a fetchResponse response to be sent from the router
type FetchResponse struct {
	ID       string `json:"id"`
	Response struct {
		Content   string `json:"content"`
		Request   string `json:"request"`
		Timestamp int32  `json:"timestamp"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//AddResponse holds an addResponse response to be sent from the router
type AddResponse struct {
	ID       string `json:"id"`
	Response struct {
		Request   string `json:"request"`
		Timestamp int32  `json:"timestamp"`
		URI       string `json:"uri"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//ListPinsResponse holds a listPinsResponse reponse to be sent from the router
type ListPinsResponse struct {
	ID       string `json:"id"`
	Response struct {
		Files     []byte `json:"files"`
		Request   string `json:"request"`
		Timestamp int32  `json:"timestamp"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//BoolResponse holds a boolResponse response to be sent from the router
type BoolResponse struct {
	ID       string `json:"id"`
	Response struct {
		OK        bool   `json:"ok"`
		Request   string `json:"request"`
		Timestamp int32  `json:"timestamp"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//ErrorResponse holds a fail response to be sent from the router
type ErrorResponse struct {
	ID    string `json:"id"`
	Error struct {
		Message   string `json:"message"`
		Request   string `json:"request"`
		Timestamp int32  `json:"timestamp"`
	} `json:"error"`
	Signature string `json:"signature"`
}

//FailBody holds a fail message to be sent from the router
/*
type FailBody struct {
	ID    string `json:"id"`
	Error struct {
		Request   string `json:"request"`
		Message   string `json:"message"`
		Timestamp int32  `json:"timestamp"`
	} `json:"error"`
	Signature string `json:"signature"`
}
*/
// CensusResponseMessage represents a census manager JSON response package
type CensusResponseMessage struct {
	ID        string         `json:"id"`
	Response  CensusResponse `json:"response"`
	Signature string         `json:"signature"`
}

// CensusResponse represents a JSON object with the response of the requested method
// Inner fields must be represented in alphabetic order
type CensusResponse struct {
	CensusID      string   `json:"censusId,omitempty"`
	ClaimsData    []string `json:"claimsData,omitempty"`
	Error         string   `json:"error,omitempty"`
	InvalidClaims []int    `json:"invalidClaims,omitempty"`
	Ok            bool     `json:"ok"`
	Request       string   `json:"request"`
	Root          string   `json:"root,omitempty"`
	Siblings      string   `json:"siblings,omitempty"`
	Timestamp     int32    `json:"timestamp"`
	URI           string   `json:"uri,omitempty"`
	ValidProof    bool     `json:"validProof,omitempty"`
}

// CensusDump represents a Dump of the census. Used for publishing on IPFS/Swarm filesystems.
type CensusDump struct {
	RootHash   string   `json:"rootHash"`
	ClaimsData []string `json:"claimsData"`
}
