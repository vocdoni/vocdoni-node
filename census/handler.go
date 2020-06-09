package census

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
)

func httpReply(resp *types.ResponseMessage, w http.ResponseWriter) {
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

// HTTPhandler handles an API census manager request via HTTP
func (m *Manager) HTTPhandler(w http.ResponseWriter, req *http.Request, signer *ethereum.SignKeys) {
	log.Debug("new request received")
	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	log.Debug("decoding JSON")
	var rm types.RequestMessage
	err := json.NewDecoder(req.Body).Decode(&rm)
	if err != nil {
		log.Warnf("cannot decode JSON: %s", err)
		http.Error(w, err.Error(), 400)
		return
	}
	if len(rm.Method) < 1 {
		http.Error(w, "method must be specified", 400)
		return
	}
	log.Debugf("found method %s", rm.Method)
	auth := true
	err = m.CheckAuth(&rm)
	if err != nil {
		log.Warnf("authorization error: %s", err)
		auth = false
	}
	resp := m.Handler(&rm.MetaRequest, auth, "")
	respMsg := new(types.ResponseMessage)
	respMsg.MetaResponse = *resp
	respMsg.ID = rm.ID
	respMsg.Request = rm.ID
	respMsg.Signature, err = signer.SignJSON(respMsg.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	httpReply(respMsg, w)
}

// Handler handles an API census manager request.
// isAuth gives access to the private methods only if censusPrefix match or censusPrefix not defined
// censusPrefix should usually be the Ethereum Address or a Hash of the allowed PubKey
func (m *Manager) Handler(r *types.MetaRequest, isAuth bool, censusPrefix string) *types.MetaResponse {
	var err error
	resp := new(types.MetaResponse)

	// Process data
	log.Debugf("processing data %+v", *r)
	resp.Ok = true
	resp.Timestamp = int32(time.Now().Unix())

	// Special methods not depending on census existence
	if r.Method == "addCensus" {
		if isAuth {
			_, err := m.AddNamespace(censusPrefix+r.CensusID, r.PubKeys)
			if err != nil {
				log.Warnf("error creating census: %s", err)
				resp.SetError(err)
			} else {
				log.Infof("census %s%s created successfully managed by %s", censusPrefix, r.CensusID, r.PubKeys)
				resp.CensusID = censusPrefix + r.CensusID
			}
		} else {
			resp.SetError("invalid authentication")
		}
		return resp
	}

	if r.Method == "getCensusList" {
		if isAuth {
			for n := range m.Trees {
				resp.CensusList = append(resp.CensusList, n)
			}
		} else {
			resp.SetError("invalid authentication")
		}
		return resp
	}

	// check if census exist
	m.TreesMu.RLock()
	exists := m.Exists(r.CensusID)
	m.TreesMu.RUnlock()
	if !exists {
		resp.SetError("censusId not valid or not found")
		return resp
	}

	// validAuthPrefix is true: either censusPrefix is not used or censusID contains the prefix
	validAuthPrefix := false
	if len(censusPrefix) == 0 {
		validAuthPrefix = true
		log.Debugf("prefix not specified, allowing access to all census IDs if pubkey validation correct")
	} else {
		validAuthPrefix = strings.HasPrefix(r.CensusID, censusPrefix)
		log.Debugf("prefix allowed for %s", r.CensusID)
	}

	// Load the merkle tree
	var tr *tree.Tree
	m.TreesMu.Lock()
	tr, err = m.LoadTree(r.CensusID)
	m.TreesMu.Unlock()
	if err != nil {
		resp.SetError("censusId cannot be loaded")
		return resp
	}

	// Methods without rootHash
	switch r.Method {
	case "getRoot":
		resp.Root = tr.Root()
		return resp

	case "addClaimBulk":
		if isAuth && validAuthPrefix {
			addedClaims := 0
			var invalidClaims []int
			for i, c := range r.ClaimsData {
				data, err := base64.StdEncoding.DecodeString(c)
				if err == nil {
					if !r.Digested {
						data = snarks.Poseidon.Hash(data)
					}
					err = tr.AddClaim(data, []byte{})
				}
				if err != nil {
					log.Warnf("error adding claim: %s", err)
					invalidClaims = append(invalidClaims, i)
				} else {
					log.Debugf("claim added %x", data)
					addedClaims++
				}
			}
			if len(invalidClaims) > 0 {
				resp.InvalidClaims = invalidClaims
			}
			log.Infof("%d claims addedd successfully", addedClaims)
		} else {
			resp.SetError("invalid authentication")
		}
		return resp

	case "addClaim":
		if isAuth && validAuthPrefix {
			data, err := base64.StdEncoding.DecodeString(r.ClaimData)
			if err != nil {
				log.Warnf("error decoding base64 string: %s", err)
				resp.SetError(err)
			}
			if !r.Digested {
				data = snarks.Poseidon.Hash(data)
			}
			err = tr.AddClaim(data, []byte{})
			if err != nil {
				log.Warnf("error adding claim: %s", err)
				resp.SetError(err)
			} else {
				log.Debugf("claim added %x", data)
			}
		} else {
			resp.SetError("invalid authentication")
		}
		return resp

	case "importDump":
		if isAuth && validAuthPrefix {
			if len(r.ClaimsData) > 0 {
				err := tr.ImportDump(r.ClaimsData)
				if err != nil {
					log.Warnf("error importing dump: %s", err)
					resp.SetError(err)
				} else {
					log.Infof("dump imported successfully, %d claims", len(r.ClaimsData))
				}
			}
		} else {
			resp.SetError("invalid authentication")
		}
		return resp

	case "importRemote":
		if !isAuth || !validAuthPrefix {
			resp.SetError("invalid authentication")
			return resp
		}
		if m.RemoteStorage == nil {
			resp.SetError("not supported")
			return resp
		}
		if !strings.HasPrefix(r.URI, m.RemoteStorage.URIprefix()) ||
			len(r.URI) <= len(m.RemoteStorage.URIprefix()) {
			log.Warnf("uri not supported %s (supported prefix %s)", r.URI, m.RemoteStorage.URIprefix())
			resp.SetError("URI not supported")
			return resp
		}
		log.Infof("retrieving remote census %s", r.CensusURI)
		censusRaw, err := m.RemoteStorage.Retrieve(context.TODO(), r.URI[len(m.RemoteStorage.URIprefix()):])
		if err != nil {
			log.Warnf("cannot retrieve census: %s", err)
			resp.SetError("cannot retrieve census")
			return resp
		}
		censusRaw = m.decompressBytes(censusRaw)
		var dump types.CensusDump
		err = json.Unmarshal(censusRaw, &dump)
		if err != nil {
			log.Warnf("retrieved census do not have a correct format: %s", err)
			resp.SetError("retrieved census do not have a correct format")
			return resp
		}
		log.Infof("retrieved census with rootHash %s and size %d bytes", dump.RootHash, len(censusRaw))
		if len(dump.ClaimsData) > 0 {
			err = tr.ImportDump(dump.ClaimsData)
			if err != nil {
				log.Warnf("error importing dump: %s", err)
				resp.SetError("error importing census")
			} else {
				log.Infof("dump imported successfully, %d claims", len(dump.ClaimsData))
			}
		} else {
			log.Warnf("no claims found on the retreived census")
			resp.SetError("no claims found")
		}
		return resp

	case "checkProof":
		if len(r.ProofData) < 1 {
			resp.SetError("proofData not provided")
			return resp
		}
		root := r.RootHash
		if len(root) < 1 {
			root = tr.Root()
		}
		// Generate proof and return it
		data, err := base64.StdEncoding.DecodeString(r.ClaimData)
		if err != nil {
			log.Warnf("error decoding base64 string: %s", err)
			resp.SetError(err)
			return resp
		}
		if !r.Digested {
			data = snarks.Poseidon.Hash(data)
		}
		validProof, err := tree.CheckProof(root, r.ProofData, data, []byte{})
		if err != nil {
			resp.SetError(err)
			return resp
		}
		resp.ValidProof = &validProof
		return resp
	}

	// Methods with rootHash, if rootHash specified snapshot the tree.
	// Otherwise, we use the same tree.
	if len(r.RootHash) > 1 {
		var err error
		tr, err = tr.Snapshot(r.RootHash)
		if err != nil {
			log.Warnf("snapshot error: %s", err)
			resp.SetError("invalid root hash")
			return resp
		}
	}

	switch r.Method {
	case "genProof":
		data, err := base64.StdEncoding.DecodeString(r.ClaimData)
		if err != nil {
			log.Warnf("error decoding base64 string: %s", err)
			resp.SetError(err)
			return resp
		}
		if !r.Digested {
			data = snarks.Poseidon.Hash(data)
		}
		resp.Siblings, err = tr.GenProof(data, []byte{})
		if err != nil {
			resp.SetError(err)
		}
		return resp

	case "getSize":
		size, err := tr.Size(tr.Root())
		if err != nil {
			resp.SetError(err)
		}
		resp.Size = &size
		return resp

	case "dump", "dumpPlain":
		if !isAuth || !validAuthPrefix {
			resp.SetError("invalid authentication")
			return resp
		}
		// dump the claim data and return it
		var dumpValues []string
		root := r.RootHash
		if len(root) < 1 {
			root = tr.Root()
		}
		var err error
		if r.Method == "dump" {
			dumpValues, err = tr.Dump(root)
		} else {
			dumpValues, _, err = tr.DumpPlain(root, true)
		}
		if err != nil {
			resp.SetError(err)
		} else {
			resp.ClaimsData = dumpValues
		}
		return resp

	case "publish":
		if !isAuth || !validAuthPrefix {
			resp.SetError("invalid authentication")
			return resp
		}
		if m.RemoteStorage == nil {
			resp.SetError("not supported")
			return resp
		}
		var dump types.CensusDump
		dump.RootHash = tr.Root()
		var err error
		dump.ClaimsData, err = tr.Dump(tr.Root())
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot dump census with root %s: %s", tr.Root(), err)
			return resp
		}
		dumpBytes, err := json.Marshal(dump)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot marshal census dump: %s", err)
			return resp
		}
		dumpBytes = m.compressBytes(dumpBytes)
		cid, err := m.RemoteStorage.Publish(context.TODO(), dumpBytes)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot publish census dump: %s", err)
			return resp
		}
		resp.URI = m.RemoteStorage.URIprefix() + cid
		log.Infof("published census at %s", resp.URI)
		resp.Root = tr.Root()

		// adding published census with censusID = rootHash
		log.Infof("adding new namespace for published census %s", resp.Root)
		tr2, err := m.AddNamespace(resp.Root, r.PubKeys)
		if err != nil && err != ErrNamespaceExist {
			log.Warnf("error creating local published census: %s", err)
		} else if err == nil {
			log.Infof("import claims to new census")
			err = tr2.ImportDump(dump.ClaimsData)
			if err != nil {
				log.Warn(err)
				resp.SetError(err)
				return resp
			}
		}
	}

	return resp
}
