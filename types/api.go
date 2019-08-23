package types

/*MessageRequest holds a decoded request but does not decode the body*/
type MessageRequest struct {
	ID        string                 `json:"id"`
	Request   map[string]interface{} `json:"request"`
	Signature string                 `json:"signature"`
}

/* the following structs hold content decoded from File API JSON objects */

//FetchFileRequest holds a fetchFile request message
type FetchFileRequest struct {
	ID      string `json:"id"`
	Request struct {
		Method    string `json:"method"`
		URI       string `json:"uri"`
		Timestamp int32  `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//AddFileRequest holds an addFile request message
type AddFileRequest struct {
	ID      string `json:"id"`
	Request struct {
		Method    string `json:"method"`
		Type      string `json:"type"`
		Content   string `json:"content"`
		Name      string `json:"name"`
		Timestamp int32  `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//PinListRequest holds a pinList request message
type PinListRequest struct {
	ID      string `json:"id"`
	Request struct {
		Method    string `json:"method"`
		Timestamp int32  `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//PinFileRequest holds a pinFile request message
type PinFileRequest struct {
	ID      string `json:"id"`
	Request struct {
		Method    string `json:"method"`
		URI       string `json:"uri"`
		Timestamp int32  `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//UnpinFileRequest holds an unpinFile request message
type UnpinFileRequest struct {
	ID      string `json:"id"`
	Request struct {
		Method    string `json:"method"`
		URI       string `json:"uri"`
		Timestamp uint   `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

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
		Request   string `json:"request"`
		Message   string `json:"message"`
		Timestamp int32  `json:"timestamp"`
	} `json:"error"`
	Signature string `json:"signature"`
}

//FailBody holds a fail message to be sent from the router
type FailBody struct {
	ID    string `json:"id"`
	Error struct {
		Request   string `json:"request"`
		Message   string `json:"message"`
		Timestamp int32  `json:"timestamp"`
	} `json:"error"`
	Signature string `json:"signature"`
}

// CensusRequestMessage represents a census manager JSON request package
type CensusRequestMessage struct {
	ID        string        `json:"id"`
	Request   CensusRequest `json:"request"`
	Signature string        `json:"signature"`
}

// CensusRequest type represents a JSON object with all possible requests fields
type CensusRequest struct {
	Method     string   `json:"method"`              // method to call
	CensusID   string   `json:"censusId"`            // References to MerkleTree namespace
	RootHash   string   `json:"rootHash"`            // References to MerkleTree rootHash
	ClaimData  string   `json:"claimData"`           // Data to add to the MerkleTree
	ClaimsData []string `json:"claimsData"`          // Multiple Data to add to the MerkleTree
	ProofData  string   `json:"proofData"`           // MerkleProof to check
	PubKeys    []string `json:"pubKeys"`             // Public key managers for creating a new census
	TimeStamp  int32    `json:"timestamp"`           // Unix TimeStamp in seconds
	CensusURI  string   `json:"censusUri,omitempty"` //Census Service URI for proxy messages
}

// CensusResponseMessage represents a census manager JSON response package
type CensusResponseMessage struct {
	ID        string         `json:"id"`
	Response  CensusResponse `json:"request"`
	Signature string         `json:"signature"`
}

// CensusResponse represents a JSON object with the response of the requested method
type CensusResponse struct {
	Ok         bool     `json:"ok"`
	Request    string   `json:"request"`
	Error      string   `json:"error"`
	Root       string   `json:"root"`
	CensusID   string   `json:"censusId"`
	Siblings   string   `json:"siblings"`
	ValidProof bool     `json:"validProof"`
	ClaimsData []string `json:"claimsData"`
	TimeStamp  int32    `json:"timestamp"`
}
