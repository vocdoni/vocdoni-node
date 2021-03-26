package census

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/snarks"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

const (
	censusHTTPhandlerTimeout   = 30 * time.Second
	censusRemoteStorageTimeout = 1 * time.Minute
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
func (m *Manager) HTTPhandler(ctx context.Context, w http.ResponseWriter,
	req *http.Request, signer *ethereum.SignKeys) {
	log.Debug("new request received")
	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	log.Debug("decoding JSON")
	var reqOuter types.RequestMessage
	if err := json.NewDecoder(req.Body).Decode(&reqOuter); err != nil {
		log.Warnf("cannot decode JSON: %s", err)
		http.Error(w, err.Error(), 400)
		return
	}
	var reqInner types.MetaRequest
	if err := json.Unmarshal(reqOuter.MetaRequest, &reqInner); err != nil {
		log.Warnf("cannot decode JSON: %s", err)
		http.Error(w, err.Error(), 400)
		return
	}
	if len(reqInner.Method) < 1 {
		http.Error(w, "method must be specified", 400)
		return
	}
	log.Debugf("found method %s", reqInner.Method)
	auth := true
	if err := m.CheckAuth(&reqOuter, &reqInner); err != nil {
		log.Warnf("authorization error: %s", err)
		auth = false
	}
	toWait := censusHTTPhandlerTimeout
	// use a bigger timeout for remote storage operations
	if req.Method == "importRemote" || req.Method == "publish" {
		toWait = censusRemoteStorageTimeout
	}
	tctx, cancel := context.WithTimeout(ctx, toWait)
	defer cancel()
	resp := m.Handler(tctx, &reqInner, auth, "")
	resp.Request = reqOuter.ID

	respInner, err := crypto.SortedMarshalJSON(resp)
	if err != nil {
		log.Errorf("cannot encode JSON: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}
	signature, err := signer.Sign(respInner)
	if err != nil {
		log.Error(err)
	}

	respOuter := &types.ResponseMessage{
		ID:           reqOuter.ID,
		Signature:    signature,
		MetaResponse: respInner,
	}
	httpReply(respOuter, w)
}

// Handler handles an API census manager request.
// isAuth gives access to the private methods only if censusPrefix match or censusPrefix not defined
// censusPrefix should usually be the Ethereum Address or a Hash of the allowed PubKey
func (m *Manager) Handler(ctx context.Context, r *types.MetaRequest, isAuth bool,
	censusPrefix string) *types.MetaResponse {
	resp := new(types.MetaResponse)

	// Process data
	log.Debugf("processing data %s", r.String())
	resp.Ok = true
	resp.Timestamp = int32(time.Now().Unix())

	// Trim Hex on censusID and RootHash
	if len(r.CensusID) > 0 {
		r.CensusID = util.TrimHex(r.CensusID)
	}

	// Special methods not depending on census existence
	if r.Method == "addCensus" {
		if isAuth {
			t, err := m.AddNamespace(censusPrefix+r.CensusID, r.PubKeys)
			if err != nil {
				log.Warnf("error creating census: %s", err)
				resp.SetError(err)
			} else {
				t.Publish()
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
		resp.SetError(fmt.Sprintf("censusId not valid or not found %s", r.CensusID))
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
	m.TreesMu.Lock()
	tr, ok := m.Trees[r.CensusID]
	m.TreesMu.Unlock()
	if !ok {
		resp.SetError("censusId cannot be loaded")
		return resp
	}
	if !tr.IsPublic() {
		resp.SetError("census not yet published")
		return resp
	}

	// Methods without rootHash
	switch r.Method {
	case "getRoot":
		resp.Root = tr.Root()
		return resp

	case "addClaimBulk":
		if isAuth && validAuthPrefix {
			invalidClaims, err := tr.AddBatch(r.CensusKeys, r.CensusValues)
			if err != nil {
				resp.SetError(err.Error())
				return resp
			}
			if len(invalidClaims) > 0 {
				resp.InvalidClaims = invalidClaims
			}
			resp.Root = tr.Root()
			log.Infof("%d claims addedd successfully", len(r.CensusKeys)-len(invalidClaims))
		} else {
			resp.SetError("invalid authentication")
		}
		return resp

	case "addClaim":
		if isAuth && validAuthPrefix {
			if r.CensusKey == nil {
				resp.SetError("error decoding claim data")
				return resp
			}
			data := r.CensusKey
			if !r.Digested {
				data = snarks.Poseidon.Hash(data)
			}
			err := tr.Add(data, r.CensusValue)
			if err != nil {
				resp.SetError(err)
			} else {
				resp.Root = tr.Root()
				log.Debugf("claim added %x/%x", data, r.CensusValue)
			}
		} else {
			resp.SetError("invalid authentication")
		}
		return resp

	case "importDump":
		if isAuth && validAuthPrefix {
			if len(r.CensusKeys) > 0 {
				err := tr.ImportDump(r.CensusDump)
				if err != nil {
					log.Warnf("error importing dump: %s", err)
					resp.SetError(err)
				} else {
					log.Infof("dump imported successfully, %d claims", len(r.CensusKeys))
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
		censusRaw, err := m.RemoteStorage.Retrieve(ctx, r.URI[len(m.RemoteStorage.URIprefix()):])
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
		log.Infof("retrieved census with rootHash %x and size %d bytes", dump.RootHash, len(censusRaw))
		if len(dump.Data) > 0 {
			err = tr.ImportDump(dump.Data)
			if err != nil {
				log.Warnf("error importing dump: %s", err)
				resp.SetError("error importing census")
			} else {
				log.Infof("dump imported successfully, %d bytes", len(dump.Data))
			}
		} else {
			log.Warnf("no data found on the retreived census")
			resp.SetError("no claims found")
		}
		return resp

	case "checkProof":
		if len(r.ProofData) < 1 {
			resp.SetError("proofData not provided")
			return resp
		}
		var err error
		var root []byte
		if len(r.RootHash) < 1 {
			root = tr.Root()
		} else {
			root = r.RootHash
		}
		// Generate proof and return it
		data := r.CensusKey
		if !r.Digested {
			data = snarks.Poseidon.Hash(data)
		}
		validProof, err := tr.CheckProof(data, r.CensusValue, root, r.ProofData)
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
			resp.SetError("cannot fetch snapshot for root")
			return resp
		}
	}

	switch r.Method {
	case "genProof":
		data := r.CensusKey
		if !r.Digested {
			data = snarks.Poseidon.Hash(data)
		}
		siblings, err := tr.GenProof(data, r.CensusValue)
		if err != nil {
			resp.SetError(err)
		}
		resp.Siblings = siblings
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
		var root []byte
		if len(r.RootHash) < 1 {
			root = tr.Root()
		} else {
			root = r.RootHash
		}
		var err error
		if r.Method == "dump" {
			resp.CensusDump, err = tr.Dump(root)
		} else {
			var vals [][]byte
			resp.CensusKeys, vals, err = tr.DumpPlain(root)
			for _, v := range vals {
				resp.CensusValues = append(resp.CensusValues, types.HexBytes(v))
			}
		}
		if err != nil {
			resp.SetError(err)
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
		dump.Data, err = tr.Dump(tr.Root())
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot dump census with root %x: %s", tr.Root(), err)
			return resp
		}
		dumpBytes, err := json.Marshal(dump)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot marshal census dump: %s", err)
			return resp
		}
		dumpBytes = m.compressBytes(dumpBytes)
		cid, err := m.RemoteStorage.Publish(ctx, dumpBytes)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot publish census dump: %s", err)
			return resp
		}
		resp.URI = m.RemoteStorage.URIprefix() + cid
		log.Infof("published census at %s", resp.URI)
		resp.Root = tr.Root()

		// adding published census with censusID = rootHash
		log.Infof("adding new namespace for published census %x", resp.Root)
		namespace := hex.EncodeToString(resp.Root)
		tr2, err := m.AddNamespace(namespace, r.PubKeys)
		if err != nil && err != ErrNamespaceExist {
			log.Warnf("error creating local published census: %s", err)
		} else if err == nil {
			log.Infof("import claims to new census")
			err = tr2.ImportDump(dump.Data)
			if err != nil {
				_ = m.DelNamespace(namespace)
				log.Warn(err)
				resp.SetError(err)
				return resp
			}
			tr2.Publish()
		}
	}
	return resp
}
