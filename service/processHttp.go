package processHttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"log"
	"github.com/vocdoni/dvote-census/tree"
)

var T tree.Tree

type Claim struct {
	CensusID string `json:"censusID"`
	ClaimData string `json:"claimData"`
	ProofData string `json:"proofData"`
}

type Result struct {
	Error bool `json:"error"`
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

func checkRequest(w http.ResponseWriter,req *http.Request) bool {
	if req.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return false
	}
	return true
}

func claimHandler(w http.ResponseWriter, req *http.Request, op string) {
	var c Claim
	var resp Result
	if ok := checkRequest(w, req); !ok { return }
	// Decode JSON
	err := json.NewDecoder(req.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Process data
	log.Printf("Received: %s,%s,%s ", c.CensusID, c.ClaimData, c.ProofData)
	resp.Error = false
	resp.Response = ""

	if len(c.CensusID) > 0 {
		T.Namespace = c.CensusID
	} else {
		resp.Error = true
		resp.Response = "CensusID is not valid"
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
		err = T.AddClaim([]byte(c.ClaimData))
	}

	if op == "gen" {
		resp.Response, err = T.GenProof([]byte(c.ClaimData))
	}

	if op == "root" {
		resp.Response = T.GetRoot()
	}

	if op == "check" {
		if len(c.ProofData) < 1 {
			resp.Error = true
			resp.Response = "proofData not provided"
			reply(&resp, w)
			return
		}
		var validProof bool
		validProof, err = T.CheckProof([]byte(c.ClaimData), c.ProofData)
		if validProof {
			resp.Response = "valid"
		} else {
			resp.Response = "invalid"
		}
	}

	if err != nil {
		resp.Error = true
		resp.Response = fmt.Sprint(err)
		log.Print(err)
		reply(&resp, w)
		return
	}

	reply(&resp, w)
}

func Listen(port int, proto string) {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: 4 * time.Second,
		ReadTimeout: 4 * time.Second,
		WriteTimeout: 4 * time.Second,
		IdleTimeout: 3 * time.Second,
	}

	http.HandleFunc("/addClaim", func(w http.ResponseWriter, r *http.Request) {
			claimHandler(w, r, "add")})
	http.HandleFunc("/genProof", func(w http.ResponseWriter, r *http.Request) {
			claimHandler(w, r, "gen")})
	http.HandleFunc("/checkProof", func(w http.ResponseWriter, r *http.Request) {
			claimHandler(w, r, "check")})
	http.HandleFunc("/getRoot", func(w http.ResponseWriter, r *http.Request) {
			claimHandler(w, r, "root")})


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
