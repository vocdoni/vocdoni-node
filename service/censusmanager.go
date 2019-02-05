package censusmanager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vocdoni/dvote-census/tree"
	"github.com/vocdoni/dvote-relay/crypto/signature"
)

const authTimeWindow = 10 // Time window (seconds) in which TimeStamp will be accepted if auth enabled

var authPubKey string

var T tree.Tree          // MerkleTree dvote-census library
var S signature.SignKeys // Signature dvote-relay library

type Claim struct {
	CensusID  string `json:"censusID"`  // References to MerkleTree namespace
	ClaimData string `json:"claimData"` // Data to add to the MerkleTree
	ProofData string `json:"proofData"` // MerkleProof to check
	TimeStamp string `json:"timeStamp"` // Unix TimeStamp in seconds
	Signature string `json:"signature"` // Signature as Hexadecimal String
}

type Result struct {
	Error    bool   `json:"error"`
	Response string `json:"response"`
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

func checkAuth(timestamp, signature, message string) bool {
	if len(authPubKey) < 1 {
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
		v, err := S.Verify(message, signature, authPubKey)
		if err != nil {
			log.Printf("Verification error: %s\n", err)
		}
		return v
	}
	return false
}

func addCorsHeaders(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func claimHandler(w http.ResponseWriter, req *http.Request, op string) {
	addCorsHeaders(&w, req)

	if (*req).Method == "OPTIONS" {
		return
	}

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
	log.Printf("Received censusID:{%s} claimData:{%s} proofData:{%s} timeStamp:{%s} signature:{%s}\n",
		c.CensusID, c.ClaimData, c.ProofData, c.TimeStamp, c.Signature)
	resp.Error = false
	resp.Response = ""

	if len(c.CensusID) > 0 {
		T.Namespace = c.CensusID
	} else {
		resp.Error = true
		resp.Response = "censusID is not valid"
		reply(&resp, w)
		return
	}

	if len(c.ClaimData) < 0 {
		resp.Error = true
		resp.Response = "data not valid"
		reply(&resp, w)
		return
	}

	if op == "add" {
		msg := fmt.Sprintf("%s%s%s", c.CensusID, c.ClaimData, c.TimeStamp)
		if auth := checkAuth(c.TimeStamp, c.Signature, msg); auth {
			if strings.HasPrefix(c.CensusID, "0x") {
				resp.Error = true
				resp.Response = "add claim to snapshot is not allowed"
			} else {
				err = T.AddClaim([]byte(c.ClaimData))
			}
		} else {
			resp.Error = true
			resp.Response = "invalid authentication"
		}
	}

	if op == "gen" {
		resp.Response, err = T.GenProof([]byte(c.ClaimData))
	}

	if op == "root" {
		resp.Response = T.GetRoot()
	}

	if op == "dump" {
		values, err := T.Dump()
		if err != nil {
			resp.Error = true
			resp.Response = fmt.Sprint(err)
		} else {
			jValues, err := json.Marshal(values)
			if err != nil {
				resp.Error = true
				resp.Response = fmt.Sprint(err)
			} else {
				resp.Response = string(jValues)
			}
		}
	}

	if op == "snapshot" {
		msg := fmt.Sprintf("%s%s%s", c.CensusID, c.ClaimData, c.TimeStamp)
		if auth := checkAuth(c.TimeStamp, c.Signature, msg); auth {
			if strings.HasPrefix(c.CensusID, "0x") {
				resp.Error = true
				resp.Response = "snapshot an snapshot makes no sense"
			} else {
				snapshotNamespace, err := T.Snapshot()
				if err != nil {
					resp.Error = true
					resp.Response = fmt.Sprint(err)
				} else {
					resp.Response = snapshotNamespace
				}
			}
		} else {
			resp.Error = true
			resp.Response = "invalid authentication"
		}
	}

	if op == "check" {
		if len(c.ProofData) < 1 {
			resp.Error = true
			resp.Response = "proofData not provided"
		} else {
			validProof, _ := T.CheckProof([]byte(c.ClaimData), c.ProofData)
			if validProof {
				resp.Response = "valid"
			} else {
				resp.Response = "invalid"
			}
		}
	}

	reply(&resp, w)
}

func Listen(port int, proto string, pubKey string) {
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
	http.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "snapshot")
	})
	http.HandleFunc("/dump", func(w http.ResponseWriter, r *http.Request) {
		claimHandler(w, r, "dump")
	})

	if len(pubKey) > 1 {
		log.Printf("Enabling signature authentication with %s\n", pubKey)
		authPubKey = pubKey
	} else {
		authPubKey = ""
	}

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
