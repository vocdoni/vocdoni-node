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

// CensusRequestMessage represents a census manager JSON request package
type CensusRequestMessage struct {
	ID        string        `json:"id"`
	Request   CensusRequest `json:"request"`
	Signature string        `json:"signature"`
}

// CensusRequest type represents a JSON object with all possible requests fields
type CensusRequest struct {
	CensusID   string   `json:"censusId"`             // References to MerkleTree namespace
	CensusURI  string   `json:"censusUri,omitempty"`  // Census Service URI for proxy messages
	ClaimData  string   `json:"claimData,omitempty"`  // Data to add to the MerkleTree
	ClaimsData []string `json:"claimsData,omitempty"` // Multiple Data to add to the MerkleTree
	Method     string   `json:"method"`               // method to call
	ProofData  string   `json:"proofData,omitempty"`  // MerkleProof to check
	PubKeys    []string `json:"pubKeys,omitempty"`    // Public key managers for creating a new census
	RootHash   string   `json:"rootHash,omitempty"`   // References to MerkleTree rootHash
	Timestamp  int32    `json:"timestamp"`            // Unix TimeStamp in seconds
	URI        string   `json:"uri,omitempty"`        // URI of a census to import/export
}

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
