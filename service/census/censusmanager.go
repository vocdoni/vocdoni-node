package censusmanager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	signature "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"
	tree "github.com/vocdoni/go-dvote/tree"
)

// Time window (seconds) in which TimeStamp will be accepted if auth enabled
const authTimeWindow = 10

// MkTrees map of merkle trees indexed by censusId
var MkTrees map[string]*tree.Tree

// Signatures map of management pubKeys indexed by censusId
var Signatures map[string]string

var currentSignature signature.SignKeys

// Claim type represents a JSON object with all possible fields
type Claim struct {
	Method    string `json:"method"`    // method to call
	CensusID  string `json:"censusId"`  // References to MerkleTree namespace
	RootHash  string `json:"rootHash"`  // References to MerkleTree rootHash
	ClaimData string `json:"claimData"` // Data to add to the MerkleTree
	ProofData string `json:"proofData"` // MerkleProof to check
	TimeStamp string `json:"timeStamp"` // Unix TimeStamp in seconds
	Signature string `json:"signature"` // Signature as Hexadecimal String
}

// Result type represents a JSON object with the result of the method executed
type Result struct {
	Error    bool   `json:"error"`
	Response string `json:"response"`
}

// AddNamespace adds a new merkletree identified by a censusId (name)
func AddNamespace(name, pubKey string) {
	if len(MkTrees) == 0 {
		MkTrees = make(map[string]*tree.Tree)
	}
	if len(Signatures) == 0 {
		Signatures = make(map[string]string)
	}

	mkTree := tree.Tree{}
	mkTree.Init(name)
	MkTrees[name] = &mkTree
	Signatures[name] = pubKey
}

func reply(resp *Result, w http.ResponseWriter) {
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), 500)
	} else {
		w.Header().Set("content-type", "application/json")
	}
}

func checkRequest(w http.ResponseWriter, req *http.Request) bool {
	if req.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return false
	}
	return true
}

func checkAuth(timestamp, signed, pubKey, message string) bool {
	if len(pubKey) < 1 {
		return true
	}
	currentTime := int64(time.Now().Unix())
	timeStampRemote, err := strconv.ParseInt(timestamp, 10, 32)
	if err != nil {
		log.Printf("Cannot parse timestamp data %s\n", err)
		return false
	}
	if timeStampRemote < currentTime+authTimeWindow &&
		timeStampRemote > currentTime-authTimeWindow {
		v, err := currentSignature.Verify(message, signed, pubKey)
		if err != nil {
			log.Printf("Verification error: %s\n", err)
		}
		return v
	}
	return false
}

func httpHandler(w http.ResponseWriter, req *http.Request) {
	var c Claim
	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	err := json.NewDecoder(req.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if len(c.Method) < 1 {
		http.Error(w, "method must be specified", 400)
		return
	}
	resp := opHandler(&c)
	reply(resp, w)
}

func opHandler(c *Claim) *Result {
	var resp Result
	op := c.Method
	var err error

	// Process data
	log.Printf("censusId:{%s} method:{%s} rootHash:{%s} claimData:{%s} proofData:{%s} timeStamp:{%s} signature:{%s}\n",
		c.CensusID, c.Method, c.RootHash, c.ClaimData, c.ProofData, c.TimeStamp, c.Signature)
	authString := fmt.Sprintf("%s%s%s%s%s", c.Method, c.CensusID, c.RootHash, c.ClaimData, c.TimeStamp)
	resp.Error = false
	resp.Response = ""
	censusFound := false
	if len(c.CensusID) > 0 {
		_, censusFound = MkTrees[c.CensusID]
	}
	if !censusFound {
		resp.Error = true
		resp.Response = "censusId not valid or not found"
		return &resp
	}

	//Methods without rootHash

	if op == "getRoot" {
		resp.Response = MkTrees[c.CensusID].GetRoot()
		return &resp
	}

	if op == "addClaim" {
		if auth := checkAuth(c.TimeStamp, c.Signature, Signatures[c.CensusID], authString); auth {
			err = MkTrees[c.CensusID].AddClaim([]byte(c.ClaimData))
		} else {
			resp.Error = true
			resp.Response = "invalid authentication"
		}
		return &resp
	}

	//Methods with rootHash, if rootHash specified snapshot the tree

	var t *tree.Tree
	if len(c.RootHash) > 1 { //if rootHash specified
		t, err = MkTrees[c.CensusID].Snapshot(c.RootHash)
		if err != nil {
			log.Printf("Snapshot error: %s", err.Error())
			resp.Error = true
			resp.Response = "invalid root hash"
			return &resp
		}
	} else { //if rootHash not specified use current tree
		t = MkTrees[c.CensusID]
	}

	if op == "genProof" {
		resp.Response, err = t.GenProof([]byte(c.ClaimData))
		if err != nil {
			resp.Error = true
			resp.Response = err.Error()
		}
		return &resp
	}

	if op == "getIdx" {
		resp.Response, err = t.GetIndex([]byte(c.ClaimData))
		return &resp
	}

	if op == "dump" {
		if auth := checkAuth(c.TimeStamp, c.Signature, Signatures[c.CensusID], authString); !auth {
			resp.Error = true
			resp.Response = "invalid authentication"
			return &resp
		}
		//dump the claim data and return it
		values, err := t.Dump()
		if err != nil {
			resp.Error = true
			resp.Response = err.Error()
		} else {
			jValues, err := json.Marshal(values)
			if err != nil {
				resp.Error = true
				resp.Response = err.Error()
			} else {
				resp.Response = fmt.Sprintf("%s", jValues)
			}
		}
		return &resp
	}

	if op == "checkProof" {
		if len(c.ProofData) < 1 {
			resp.Error = true
			resp.Response = "proofData not provided"
			return &resp
		}
		// Generate proof and return it
		validProof, err := t.CheckProof([]byte(c.ClaimData), c.ProofData)
		if err != nil {
			resp.Error = true
			resp.Response = err.Error()
			return &resp
		}
		if validProof {
			resp.Response = "valid"
		} else {
			resp.Response = "invalid"
		}
		return &resp
	}

	return &resp
}

func addCorsHeaders(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// Listen starts a census service defined of type "proto"
func Listen(port int, proto string) {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: 4 * time.Second,
		ReadTimeout:       4 * time.Second,
		WriteTimeout:      4 * time.Second,
		IdleTimeout:       3 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		addCorsHeaders(&w, r)
		if r.Method == http.MethodPost {
			httpHandler(w, r)
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
	if proto == "https" {
		log.Print("Starting server in https mode")
		if err := srv.ListenAndServeTLS("server.crt", "server.key"); err != nil {
			panic(err)
		}
	}
	if proto == "http" {
		log.Print("Starting server in http mode")
		srv.SetKeepAlivesEnabled(false)
		if err := srv.ListenAndServe(); err != nil {
			panic(err)
		}
	}
}
