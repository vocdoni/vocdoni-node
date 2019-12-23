// Package census provides the census management operation
package census

import (
	"encoding/base64"
	"encoding/json"

	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/types"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/tree"
)

type Namespaces struct {
	RootKey    string      `json:"rootKey"` // Public key allowed to created new census
	Namespaces []Namespace `json:"namespaces"`
}

type Namespace struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

type Manager struct {
	Storage     string                // Root storage data dir
	AuthWindow  int32                 // Time window (seconds) in which TimeStamp will be accepted if auth enabled
	Census      Namespaces            // Available namespaces
	Trees       map[string]*tree.Tree // MkTrees map of merkle trees indexed by censusId
	Data        data.Storage
	ImportQueue map[string]string
}

// Init creates a new census manager
func (m *Manager) Init(storage, rootKey string) error {
	nsConfig := fmt.Sprintf("%s/namespaces.json", storage)
	m.Storage = storage
	m.Trees = make(map[string]*tree.Tree)
	m.ImportQueue = make(map[string]string)
	m.AuthWindow = 10

	log.Infof("loading namespaces and keys from %s", nsConfig)
	if _, err := os.Stat(nsConfig); os.IsNotExist(err) {
		log.Info("creating new config file")
		var cns Namespaces
		if len(rootKey) < signature.PubKeyLength {
			// log.Warn("no root key provided or invalid, anyone will be able to create new census")
			cns.RootKey = ""
		} else {
			cns.RootKey = rootKey
		}
		m.Census = cns
		ioutil.WriteFile(nsConfig, []byte(""), 0644)
		err = m.save()
		return err
	}

	jsonBytes, err := ioutil.ReadFile(nsConfig)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonBytes, &m.Census); err != nil {
		log.Warn("could not unmarshal json config file, probably empty. Skipping")
		return nil
	}
	if len(rootKey) >= signature.PubKeyLength {
		log.Infof("updating root key to %s", rootKey)
		m.Census.RootKey = rootKey
	} else {
		if rootKey != "" {
			log.Infof("current root key %s", rootKey)
		}
	}
	// Initialize existing merkle trees
	for _, ns := range m.Census.Namespaces {
		t := tree.Tree{}
		t.Storage = m.Storage
		err := t.Init(ns.Name)
		if err != nil {
			log.Warn(err)
		} else {
			log.Infof("initialized merkle tree %s", ns.Name)
			m.Trees[ns.Name] = &t
		}
	}
	// Start daemon for importing remote census
	go m.ImportQueueDaemon()

	return nil
}

// AddNamespace adds a new merkletree identified by a censusId (name)
func (m *Manager) AddNamespace(name string, pubKeys []string) error {
	if _, e := m.Trees[name]; e {
		return errors.New("namespace already exist")
	}
	mkTree := tree.Tree{}
	mkTree.Storage = m.Storage
	err := mkTree.Init(name)
	if err != nil {
		return err
	}
	m.Trees[name] = &mkTree
	var ns Namespace
	ns.Name = name
	ns.Keys = pubKeys
	m.Census.Namespaces = append(m.Census.Namespaces, ns)
	return m.save()
}

// DelNamespace removes a merkletree namespace
func (m *Manager) DelNamespace(name string) error {
	if _, e := m.Trees[name]; e {
		delete(m.Trees, name)
		os.RemoveAll(m.Storage + "/" + name)
	}
	for i, ns := range m.Census.Namespaces {
		if ns.Name == name {
			m.Census.Namespaces = m.Census.Namespaces[:i+
				copy(m.Census.Namespaces[i:], m.Census.Namespaces[i+1:])]
			break
		}
	}
	return m.save()
}

func (m *Manager) save() error {
	log.Info("saving namespaces")
	nsConfig := fmt.Sprintf("%s/namespaces.json", m.Storage)
	data, err := json.Marshal(m.Census)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(nsConfig, data, 0644)
}

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

// CheckAuth check if a census request message is authorized
func (m *Manager) CheckAuth(rm *types.RequestMessage) error {
	if len(rm.Signature) < signature.SignatureLength || len(rm.CensusID) < 1 {
		return errors.New("signature or censusId not provided or invalid")
	}
	ns := new(Namespace)
	for _, n := range m.Census.Namespaces {
		if n.Name == rm.CensusID {
			ns = &n
		}
	}

	// Add root key, if method is addCensus
	if rm.Method == "addCensus" {
		if len(m.Census.RootKey) < signature.PubKeyLength {
			log.Warn("root key does not exist, considering addCensus valid for any request")
			return nil
		}
		ns.Keys = []string{m.Census.RootKey}
	}

	if ns == nil {
		return errors.New("censusId not valid")
	}

	// Check timestamp
	currentTime := int32(time.Now().Unix())
	if rm.Timestamp > currentTime+m.AuthWindow ||
		rm.Timestamp < currentTime-m.AuthWindow {
		return errors.New("timestamp is not valid")
	}

	// Check signature with existing namespace keys
	log.Debugf("namespace keys %s", ns.Keys)
	if len(ns.Keys) > 0 {
		if len(ns.Keys) == 1 && len(ns.Keys[0]) < signature.PubKeyLength {
			log.Warnf("namespace %s does have management public key configured, allowing all", ns.Name)
			return nil
		}
		valid := false
		msg, err := json.Marshal(rm.MetaRequest)
		if err != nil {
			return errors.New("cannot unmarshal")
		}
		for _, n := range ns.Keys {
			valid, err = signature.Verify(string(msg), rm.Signature, n)
			if err != nil {
				log.Warnf("verification error (%s)", err)
				valid = false
			} else if valid {
				return nil
			}
		}
		if !valid {
			return errors.New("unauthorized")
		}
	} else {
		log.Warnf("namespace %s does have management public key configured, allowing all", ns.Name)
	}
	return nil
}

// HTTPhandler handles an API census manager request via HTTP
func (m *Manager) HTTPhandler(w http.ResponseWriter, req *http.Request, signer *signature.SignKeys) {
	log.Debug("new request received")
	var rm types.RequestMessage
	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	log.Debug("decoding JSON")
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

// AddToImportQueue adds a new census to the queue for being imported remotelly
func (m *Manager) AddToImportQueue(censusID string, censusURI string) {
	m.ImportQueue[censusID] = censusURI
}

// ImportQueueDaemon fetch and import remote census added on ImportQueue
func (m *Manager) ImportQueueDaemon() {
	log.Info("starting import queue daemon")
	for {
		for cid, uri := range m.ImportQueue {
			if _, ok := m.Trees[cid]; ok {
				log.Debugf("census %s already exist, skipping", cid)
				delete(m.ImportQueue, cid)
				continue
			}
			log.Infof("retrieving remote census %s", uri)
			censusRaw, err := m.Data.Retrieve(uri[len(m.Data.URIprefix()):])
			if err != nil {
				log.Warnf("cannot retrieve census: %s", err)
				delete(m.ImportQueue, cid)
				continue
			}
			var dump types.CensusDump
			err = json.Unmarshal(censusRaw, &dump)
			if err != nil {
				log.Warnf("retrieved census does not have a valid format: %s", err)
				delete(m.ImportQueue, cid)
				continue
			}
			log.Infof("retrieved census with rootHash %s and size %d bytes", dump.RootHash, len(censusRaw))
			if dump.RootHash != cid {
				log.Warn("dump root Hash and Ethereum root hash do not match, aborting import")
				delete(m.ImportQueue, cid)
				continue
			}
			if len(dump.ClaimsData) > 0 {
				if err := m.AddNamespace(cid, []string{}); err != nil {
					log.Errorf("cannot create new census namespace: %s", err)
					continue
				}
				err = m.Trees[cid].ImportDump(dump.ClaimsData)
				if err != nil {
					log.Warnf("error importing dump: %s", err)
					delete(m.ImportQueue, cid)
					continue
				} else {
					log.Infof("dump imported successfully, %d claims", len(dump.ClaimsData))
				}
				if m.Trees[cid].Root() != dump.RootHash {
					log.Warnf("root hash does not match on imported census, aborting import")
					if err := m.DelNamespace(cid); err != nil {
						log.Error(err)
					}
				}
			} else {
				log.Warnf("no claims found on the retreived census")
			}
			delete(m.ImportQueue, cid)
		}
		time.Sleep(time.Second * 1)
	}
}

// Handler handles an API census manager request.
// isAuth gives access to the private methods only if censusPrefix match or censusPrefix not defined
// censusPrefix should usually be the Ethereum Address or a Hash of the allowed PubKey
func (m *Manager) Handler(r *types.MetaRequest, isAuth bool, censusPrefix string) *types.MetaResponse {
	resp := new(types.MetaResponse)
	op := r.Method
	var err error

	// Process data
	log.Debugf("processing data %+v", *r)
	resp.Ok = true
	resp.Timestamp = int32(time.Now().Unix())

	if op == "addCensus" {
		if isAuth {
			err = m.AddNamespace(censusPrefix+r.CensusID, r.PubKeys)
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

	censusFound := false
	for k := range m.Trees {
		if k == r.CensusID {
			censusFound = true
			break
		}
	}
	if !censusFound {
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

	// Methods without rootHash
	switch op {
	case "getRoot":
		resp.Root = m.Trees[r.CensusID].Root()
		return resp

	case "addClaimBulk":
		if isAuth && validAuthPrefix {
			addedClaims := 0
			var invalidClaims []int
			for i, c := range r.ClaimsData {
				data, err := base64.StdEncoding.DecodeString(c)
				if err == nil {
					err = m.Trees[r.CensusID].AddClaim(data)
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
			err = m.Trees[r.CensusID].AddClaim(data)
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
				err = m.Trees[r.CensusID].ImportDump(r.ClaimsData)
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
		// To-Do implement Gzip compression
		if !isAuth || !validAuthPrefix {
			resp.SetError("invalid authentication")
			return resp
		}
		if m.Data == nil {
			resp.SetError("not supported")
			return resp
		}
		if !strings.HasPrefix(r.URI, m.Data.URIprefix()) ||
			len(r.URI) <= len(m.Data.URIprefix()) {
			log.Warnf("uri not supported %s (supported prefix %s)", r.URI, m.Data.URIprefix())
			resp.SetError("URI not supported")
			return resp
		}
		log.Infof("retrieving remote census %s", r.CensusURI)
		censusRaw, err := m.Data.Retrieve(r.URI[len(m.Data.URIprefix()):])
		if err != nil {
			log.Warnf("cannot retrieve census: %s", err)
			resp.SetError("cannot retrieve census")
			return resp
		}
		var dump types.CensusDump
		err = json.Unmarshal(censusRaw, &dump)
		if err != nil {
			log.Warnf("retrieved census do not have a correct format: %s", err)
			resp.SetError("retrieved census do not have a correct format")
			return resp
		}
		log.Infof("retrieved census with rootHash %s and size %d bytes", dump.RootHash, len(censusRaw))
		if len(dump.ClaimsData) > 0 {
			err = m.Trees[r.CensusID].ImportDump(dump.ClaimsData)
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
			root = m.Trees[r.CensusID].Root()
		}
		// Generate proof and return it
		data, err := base64.StdEncoding.DecodeString(r.ClaimData)
		if err != nil {
			log.Warnf("error decoding base64 string: %s", err)
			resp.SetError(err)
			return resp
		}
		validProof, err := tree.CheckProof(root, r.ProofData, data)
		if err != nil {
			resp.SetError(err)
			return resp
		}
		resp.ValidProof = validProof
		return resp
	}

	// Methods with rootHash, if rootHash specified snapshot the tree
	var t *tree.Tree
	if len(r.RootHash) > 1 { // if rootHash specified
		t, err = m.Trees[r.CensusID].Snapshot(r.RootHash)
		if err != nil {
			log.Warnf("snapshot error: %s", err)
			resp.SetError("invalid root hash")
			return resp
		}
	} else { // if rootHash not specified use current tree
		t = m.Trees[r.CensusID]
	}

	switch op {
	case "genProof":
		data, err := base64.StdEncoding.DecodeString(r.ClaimData)
		if err != nil {
			log.Warnf("error decoding base64 string: %s", err)
			resp.SetError(err)
			return resp
		}
		resp.Siblings, err = t.GenProof(data)
		if err != nil {
			resp.SetError(err)
		}
		return resp

	case "getSize":
		resp.Size, err = t.Size(t.Root())
		if err != nil {
			resp.SetError(err)
		}
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
			root = t.Root()
		}
		if op == "dump" {
			dumpValues, err = t.Dump(root)
		} else {
			dumpValues, err = t.DumpPlain(root, true)
		}
		if err != nil {
			resp.SetError(err)
		} else {
			resp.ClaimsData = dumpValues
		}
		return resp

	case "publish":
		// To-Do implement Gzip compression
		if !isAuth || !validAuthPrefix {
			resp.SetError("invalid authentication")
			return resp
		}
		if m.Data == nil {
			resp.SetError("not supported")
			return resp
		}
		var dump types.CensusDump
		dump.RootHash = t.Root()
		dump.ClaimsData, err = t.Dump(t.Root())
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot dump census with root %s: %s", t.Root(), err)
			return resp
		}
		dumpBytes, err := json.Marshal(dump)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot marshal census dump: %s", err)
			return resp
		}
		cid, err := m.Data.Publish(dumpBytes)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot publish census dump: %s", err)
			return resp
		}
		resp.URI = m.Data.URIprefix() + cid
		log.Infof("published census at %s", resp.URI)
		resp.Root = t.Root()

		// adding published census with censusID = rootHash
		log.Infof("adding new namespace for published census %s", resp.Root)
		err = m.AddNamespace(resp.Root, r.PubKeys)
		if err != nil {
			log.Warnf("error creating local published census: %s", err)
		} else {
			log.Infof("import claims to new census")
			err = m.Trees[resp.Root].ImportDump(dump.ClaimsData)
			if err != nil {
				log.Warn(err)
			}
		}
	}

	return resp
}
