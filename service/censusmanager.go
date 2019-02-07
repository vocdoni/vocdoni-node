package censusmanager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	tree "github.com/vocdoni/dvote-census/tree"
	signature "github.com/vocdoni/dvote-relay/crypto/signature"
)

const hashSize = 32
const authTimeWindow = 10         // Time window (seconds) in which TimeStamp will be accepted if auth enabled
var MkTrees map[string]*tree.Tree // MerkleTree dvote-census library
var Signatures map[string]string
var Signature signature.SignKeys // Signature dvote-relay library

type Claim struct {
	CensusID  string `json:"censusID"`  // References to MerkleTree namespace
	RootHash  string `json:"rootHash"`  // References to MerkleTree rootHash
	ClaimData string `json:"claimData"` // Data to add to the MerkleTree
	ProofData string `json:"proofData"` // MerkleProof to check
	TimeStamp string `json:"timeStamp"` // Unix TimeStamp in seconds
	Signature string `json:"signature"` // Signature as Hexadecimal String
}

type Result struct {
	Error    bool   `json:"error"`
	Response string `json:"response"`
}

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

func checkAuth(timestamp, signature, pubKey, message string) bool {
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
		v, err := Signature.Verify(message, signature, pubKey)
		if err != nil {
			log.Printf("Verification error: %s\n", err)
		}
		return v
	}
	return false
}

func claimHandler(w http.ResponseWriter, req *http.Request, op string) {
	var c Claim
	var resp Result

	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	err := json.NewDecoder(req.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Process data
	log.Printf("censusID:{%s} rootHash:{%s} claimData:{%s} proofData:{%s} timeStamp:{%s} signature:{%s}\n",
		c.CensusID, c.RootHash, c.ClaimData, c.ProofData, c.TimeStamp, c.Signature)
	authString := fmt.Sprintf("%s%s%s%s", c.CensusID, c.RootHash, c.ClaimData, c.TimeStamp)
	resp.Error = false
	resp.Response = ""
	censusFound := false
	if len(c.CensusID) > 0 {
		_, censusFound = MkTrees[c.CensusID]
	}
	if !censusFound {
		resp.Error = true
		resp.Response = "censusID not valid or not found"
		reply(&resp, w)
		return
	}

	if op == "add" {
		if auth := checkAuth(c.TimeStamp, c.Signature, Signatures[c.CensusID], authString); auth {
			err = MkTrees[c.CensusID].AddClaim([]byte(c.ClaimData))
		} else {
			resp.Error = true
			resp.Response = "invalid authentication"
		}
	}

	if op == "gen" {
		var t *tree.Tree
		var err error
		if len(c.RootHash) > 1 { //if rootHash specified
			t, err = MkTrees[c.CensusID].Snapshot(c.RootHash)
			if err != nil {
				log.Printf("Snapshot error: %s", err.Error())
				resp.Error = true
				resp.Response = "invalid root hash"
				reply(&resp, w)
				return
			}
		} else { //if rootHash not specified use current tree
			t = MkTrees[c.CensusID]
		}
		resp.Response, err = t.GenProof([]byte(c.ClaimData))
		if err != nil {
			resp.Error = true
			resp.Response = err.Error()
			reply(&resp, w)
			return
		}
	}

	if op == "root" {
		resp.Response = MkTrees[c.CensusID].GetRoot()
	}

	if op == "idx" {

	}

	if op == "dump" {
		var t *tree.Tree
		if auth := checkAuth(c.TimeStamp, c.Signature, Signatures[c.CensusID], authString); !auth {
			resp.Error = true
			resp.Response = "invalid authentication"
			reply(&resp, w)
			return
		}

		if len(c.RootHash) > 1 { //if rootHash specified
			t, err = MkTrees[c.CensusID].Snapshot(c.RootHash)
			if err != nil {
				log.Printf("Snapshot error: %s", err.Error())
				resp.Error = true
				resp.Response = "invalid root hash"
				reply(&resp, w)
				return
			}
		} else { //if rootHash not specified use current merkletree
			t = MkTrees[c.CensusID]
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
				resp.Response = string(jValues)
			}
		}
	}

	if op == "check" {
		if len(c.ProofData) < 1 {
			resp.Error = true
			resp.Response = "proofData not provided"
			reply(&resp, w)
			return
		}
		var t *tree.Tree
		if len(c.RootHash) > 1 { //if rootHash specified
			t, err = MkTrees[c.CensusID].Snapshot(c.RootHash)
			if err != nil {
				log.Printf("Snapshot error: %s", err.Error())
				resp.Error = true
				resp.Response = "invalid root hash"
				reply(&resp, w)
				return
			}
		} else { //if rootHash not specified use current merkletree
			t = MkTrees[c.CensusID]
		}

		validProof, err := t.CheckProof([]byte(c.ClaimData), c.ProofData)
		if err != nil {
			resp.Error = true
			resp.Response = err.Error()
			reply(&resp, w)
			return
		}
		if validProof {
			resp.Response = "valid"
		} else {
			resp.Response = "invalid"
		}
	}

	reply(&resp, w)
}

func Listen(port int, proto string) {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: 4 * time.Second,
		ReadTimeout:       4 * time.Second,
		WriteTimeout:      4 * time.Second,
		IdleTimeout:       3 * time.Second,
	}

	http.HandleFunc("/addClaim", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "add")
	})
	http.HandleFunc("/genProof", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "gen")
	})
	http.HandleFunc("/checkProof", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "check")
	})
	http.HandleFunc("/getRoot", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "root")
	})
	http.HandleFunc("/dump", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "dump")
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
