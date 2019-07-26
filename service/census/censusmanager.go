package censusmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitlab.com/vocdoni/go-dvote/types"

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	tree "gitlab.com/vocdoni/go-dvote/tree"
)

// Time window (seconds) in which TimeStamp will be accepted if auth enabled
const authTimeWindow = 10

// MkTrees map of merkle trees indexed by censusId
var MkTrees map[string]*tree.Tree

// Signatures map of management pubKeys indexed by censusId
var Signatures map[string]string

var currentSignature signature.SignKeys

// AddNamespace adds a new merkletree identified by a censusId (name)
func AddNamespace(name, pubKey string) {
	if len(MkTrees) == 0 {
		MkTrees = make(map[string]*tree.Tree)
	}
	if len(Signatures) == 0 {
		Signatures = make(map[string]string)
	}
	log.Infof("Adding namespace %s", name)
	mkTree := tree.Tree{}
	mkTree.Init(name)
	MkTrees[name] = &mkTree
	Signatures[name] = pubKey
}

func reply(resp *types.CensusResponseMessage, w http.ResponseWriter) {
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

func checkAuth(timestamp int32, signed, pubKey, message string) bool {
	if len(pubKey) < 1 {
		return true
	}
	currentTime := int32(time.Now().Unix())
	if timestamp < currentTime+authTimeWindow &&
		timestamp > currentTime-authTimeWindow {
		v, err := currentSignature.Verify(message, signed, pubKey)
		if err != nil {
			log.Warnf("Verification error: %s\n", err)
		}
		return v
	}
	return false
}

func httpHandler(w http.ResponseWriter, req *http.Request, signer *signature.SignKeys) {
	log.Debug("New request received")
	var rm types.CensusRequestMessage
	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	log.Debug("Decoding JSON")

	/*
		buf := new(bytes.Buffer)
		buf.ReadFrom(req.Body)
		reqStr := buf.String()
		log.Debug(reqStr)
	*/
	err := json.NewDecoder(req.Body).Decode(&rm)
	if err != nil {
		log.Warnf("Cannot decode JSON: %s", err.Error())
		http.Error(w, err.Error(), 400)
		return
	}
	if len(rm.Request.Method) < 1 {
		http.Error(w, "method must be specified", 400)
		return
	}
	log.Debugf("Found method %s", rm.Request.Method)
	resp := opHandler(&rm.Request, true)
	respMsg := new(types.CensusResponseMessage)
	respMsg.Response = *resp
	respMsg.Signature = "0x0"
	respMsg.ID = rm.ID
	respMsg.Response.Request = rm.ID
	respMsg.Signature, err = signer.SignJSON(respMsg.Response)
	if err != nil {
		log.Warn(err.Error())
	}
	reply(respMsg, w)
}

func opHandler(r *types.CensusRequest, isAuth bool) *types.CensusResponse {
	resp := new(types.CensusResponse)
	op := r.Method
	var err error

	// Process data
	log.Infof("Processing data => %+v", *r)

	resp.Ok = true
	resp.Error = ""
	resp.TimeStamp = int32(time.Now().Unix())
	censusFound := false
	for k := range MkTrees {
		if k == r.CensusID {
			censusFound = true
			break
		}
	}
	if !censusFound {
		resp.Ok = false
		resp.Error = "censusId not valid or not found"
		return resp
	}

	//Methods without rootHash

	if op == "getRoot" {
		resp.Root = MkTrees[r.CensusID].GetRoot()
		return resp
	}

	if op == "addClaim" {
		if isAuth {
			err = MkTrees[r.CensusID].AddClaim([]byte(r.ClaimData))
			if err != nil {
				log.Warnf("error adding claim: %s", err.Error())
				resp.Ok = false
				resp.Error = err.Error()
			} else {
				log.Info("claim addedd successfully ")
			}
		} else {
			resp.Ok = false
			resp.Error = "invalid authentication"
		}
		return resp
	}

	//Methods with rootHash, if rootHash specified snapshot the tree

	var t *tree.Tree
	if len(r.RootHash) > 1 { //if rootHash specified
		t, err = MkTrees[r.CensusID].Snapshot(r.RootHash)
		if err != nil {
			log.Warnf("Snapshot error: %s", err.Error())
			resp.Ok = false
			resp.Error = "invalid root hash"
			return resp
		}
	} else { //if rootHash not specified use current tree
		t = MkTrees[r.CensusID]
	}

	if op == "genProof" {
		resp.Siblings, err = t.GenProof([]byte(r.ClaimData))
		if err != nil {
			resp.Ok = false
			resp.Error = err.Error()
		}
		return resp
	}

	if op == "getIdx" {
		resp.Idx, err = t.GetIndex([]byte(r.ClaimData))
		return resp
	}

	if op == "dump" {
		if !isAuth {
			resp.Ok = false
			resp.Error = "invalid authentication"
			return resp
		}
		//dump the claim data and return it
		values, err := t.Dump()
		if err != nil {
			resp.Ok = false
			resp.Error = err.Error()
		} else {
			resp.ClaimsData = values
		}
		return resp
	}

	if op == "checkProof" {
		if len(r.ProofData) < 1 {
			resp.Ok = false
			resp.Error = "proofData not provided"
			return resp
		}
		// Generate proof and return it
		validProof, err := t.CheckProof([]byte(r.ClaimData), r.ProofData)
		if err != nil {
			resp.Ok = false
			resp.Error = err.Error()
			return resp
		}
		if validProof {
			resp.ValidProof = true
		} else {
			resp.ValidProof = false
		}
		return resp
	}

	return resp
}

func addCorsHeaders(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// Listen starts a census service defined of type "proto"
func Listen(port int, proto string, signer *signature.SignKeys) {
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
			httpHandler(w, r, signer)
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
	if proto == "https" {
		log.Infof("Starting server in https mode")
		if err := srv.ListenAndServeTLS("server.crt", "server.key"); err != nil {
			log.Panic(err)
		}
	}
	if proto == "http" {
		log.Infof("Starting server in http mode")
		srv.SetKeepAlivesEnabled(false)
		if err := srv.ListenAndServe(); err != nil {
			log.Panic(err)
		}
	}
}
