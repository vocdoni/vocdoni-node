// Package census provides the census management operation
package census

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
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
	StorageDir    string        // Root storage data dir
	AuthWindow    int32         // Time window (seconds) in which TimeStamp will be accepted if auth enabled
	Census        Namespaces    // Available namespaces
	LoadThreshold time.Duration // How many time will a Census stay open/loaded since the last access
	// TODO(mvdan): should we protect Census with the mutex too?
	TreesMu sync.RWMutex
	Trees   map[string]*tree.Tree // MkTrees map of merkle trees indexed by censusId

	Storage data.Storage

	importQueue chan censusImport

	compressor
}

type censusImport struct {
	censusID, censusURI string
}

// Data helps satisfy an ethevents interface.
func (m *Manager) Data() data.Storage { return m.Storage }

// Init creates a new census manager
func (m *Manager) Init(storage, rootKey string) error {
	nsConfig := fmt.Sprintf("%s/namespaces.json", storage)
	m.StorageDir = storage
	m.Trees = make(map[string]*tree.Tree)
	m.LoadThreshold = 1200 * time.Second

	// add a bit of buffering, to try to keep AddToImportQueue non-blocking.
	m.importQueue = make(chan censusImport, 32)
	m.AuthWindow = 10

	m.compressor = newCompressor()

	log.Infof("loading namespaces and keys from %s", nsConfig)
	if _, err := os.Stat(nsConfig); os.IsNotExist(err) {
		log.Info("creating new config file")
		var cns Namespaces
		if len(rootKey) < signature.PubKeyLength {
			// log.Warn("no root key provided or invalid, anyone will be able to create new census")
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
	// Start daemon for importing remote census
	go m.importQueueDaemon()
	// Start daemon for unloading trees
	go m.unloadManager()

	return nil
}

// LoadTree opens the database containing the merkle tree or returns nil if already loaded
// Not thread safe
func (m *Manager) LoadTree(name string) (*tree.Tree, error) {
	if _, exist := m.Trees[name]; exist {
		return m.Trees[name], nil
	}
	tr := &tree.Tree{}
	tr.StorageDir = m.StorageDir
	if err := tr.Init(name); err != nil {
		return nil, err
	}
	log.Infof("load merkle tree %s", name)
	m.Trees[name] = tr
	return tr, nil
}

// UnloadTree closes the database containing the merkle tree
// Not thread safe
func (m *Manager) UnloadTree(name string) {
	log.Debugf("unload merkle tree %s", name)
	m.Trees[name].Close()
	delete(m.Trees, name)
	return
}

// Exists returns true if a given census exists on disk
// While Exists() means there is a tree database with such name,
//  Load() reads the tree from disk and create the required memory structure in order to use it
// Not thread safe, Mutex must be controlled on the calling function
func (m *Manager) Exists(name string) bool {
	for _, ns := range m.Census.Namespaces {
		if name == ns.Name {
			return true
		}
	}
	return false
}

// AddNamespace adds a new merkletree identified by a censusId (name), and
// returns the new tree.
func (m *Manager) AddNamespace(name string, pubKeys []string) (*tree.Tree, error) {
	m.TreesMu.Lock()
	defer m.TreesMu.Unlock()
	if m.Exists(name) {
		return nil, errors.New("namespace already exist")
	}
	tr := &tree.Tree{
		StorageDir: m.StorageDir,
	}
	if err := tr.Init(name); err != nil {
		return nil, err
	}
	m.Trees[name] = tr
	m.Census.Namespaces = append(m.Census.Namespaces, Namespace{
		Name: name,
		Keys: pubKeys,
	})
	return tr, m.save()
}

// DelNamespace removes a merkletree namespace
func (m *Manager) DelNamespace(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("no valid namespace provided")
	}
	m.TreesMu.Lock()
	defer m.TreesMu.Unlock()
	if !m.Exists(name) {
		return nil
	}
	m.UnloadTree(name)
	if err := os.RemoveAll(m.StorageDir + "/" + name); err != nil {
		return fmt.Errorf("cannot remove census: (%s)", err)
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
	nsConfig := fmt.Sprintf("%s/namespaces.json", m.StorageDir)
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

// unloadManager is a blocking process that unloads trees if they are not used for some time
func (m *Manager) unloadManager() {
	for {
		time.Sleep(60 * time.Second)
		m.TreesMu.Lock()
		for k, tree := range m.Trees {
			if tree.LastAccess().Add(m.LoadThreshold).Before(time.Now()) {
				m.UnloadTree(k)
			}
		}
		m.TreesMu.Unlock()
	}
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

// AddToImportQueue adds a new census to the queue for being imported remotelly
func (m *Manager) AddToImportQueue(censusID, censusURI string) {
	m.importQueue <- censusImport{censusID: censusID, censusURI: censusURI}
}

// ImportQueueDaemon fetches and imports remote census added via importQueue.
func (m *Manager) importQueueDaemon() {
	log.Info("starting import queue daemon")
	for imp := range m.importQueue {
		cid, uri := imp.censusID, imp.censusURI
		// TODO(mvdan): this lock is separate from the one
		// from AddNamespace below. The namespace might appear
		// in between the two pieces of code.
		m.TreesMu.RLock()
		exists := m.Exists(cid)
		m.TreesMu.RUnlock()
		if exists {
			log.Debugf("census %s already exist, skipping", cid)
			continue
		}
		log.Infof("retrieving remote census %s", uri)
		censusRaw, err := m.Storage.Retrieve(context.TODO(), uri[len(m.Storage.URIprefix()):])
		if err != nil {
			log.Warnf("cannot retrieve census: %s", err)
			continue
		}
		censusRaw = m.decompressBytes(censusRaw)
		var dump types.CensusDump
		if err := json.Unmarshal(censusRaw, &dump); err != nil {
			log.Warnf("retrieved census does not have a valid format: %s", err)
			continue
		}
		log.Infof("retrieved census with rootHash %s and size %d bytes", dump.RootHash, len(censusRaw))
		if dump.RootHash != cid {
			log.Warn("dump root Hash and Ethereum root hash do not match, aborting import")
			continue
		}
		if len(dump.ClaimsData) == 0 {
			log.Warnf("no claims found on the retreived census")
			continue
		}
		tr, err := m.AddNamespace(cid, []string{})
		if err != nil {
			log.Errorf("cannot create new census namespace: %s", err)
			continue
		}
		err = tr.ImportDump(dump.ClaimsData)
		if err != nil {
			log.Warnf("error importing dump: %s", err)
			continue
		}
		log.Infof("dump imported successfully, %d claims", len(dump.ClaimsData))
		if tr.Root() != dump.RootHash {
			log.Warnf("root hash does not match on imported census, aborting import")
			if err := m.DelNamespace(cid); err != nil {
				log.Error(err)
			}
		} else {
			// Unload until we need it
			m.TreesMu.Lock()
			m.UnloadTree(cid)
			m.TreesMu.Unlock()
		}
	}
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
						data = signature.HashPoseidon(fmt.Sprintf("%x", data))
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
				data = signature.HashPoseidon(fmt.Sprintf("%x", data))
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
		if m.Storage == nil {
			resp.SetError("not supported")
			return resp
		}
		if !strings.HasPrefix(r.URI, m.Storage.URIprefix()) ||
			len(r.URI) <= len(m.Storage.URIprefix()) {
			log.Warnf("uri not supported %s (supported prefix %s)", r.URI, m.Storage.URIprefix())
			resp.SetError("URI not supported")
			return resp
		}
		log.Infof("retrieving remote census %s", r.CensusURI)
		censusRaw, err := m.Storage.Retrieve(context.TODO(), r.URI[len(m.Storage.URIprefix()):])
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
			data = signature.HashPoseidon(fmt.Sprintf("%x", data))
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
			data = signature.HashPoseidon(fmt.Sprintf("%x", data))
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
		if m.Storage == nil {
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
		cid, err := m.Storage.Publish(context.TODO(), dumpBytes)
		if err != nil {
			resp.SetError(err)
			log.Warnf("cannot publish census dump: %s", err)
			return resp
		}
		resp.URI = m.Storage.URIprefix() + cid
		log.Infof("published census at %s", resp.URI)
		resp.Root = tr.Root()

		// adding published census with censusID = rootHash
		log.Infof("adding new namespace for published census %s", resp.Root)
		tr2, err := m.AddNamespace(resp.Root, r.PubKeys)
		if err != nil {
			log.Warnf("error creating local published census: %s", err)
		} else {
			log.Infof("import claims to new census")
			err = tr2.ImportDump(dump.ClaimsData)
			if err != nil {
				log.Warn(err)
			}
		}
	}

	return resp
}

type compressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

func newCompressor() compressor {
	var c compressor
	var err error
	c.encoder, err = zstd.NewWriter(nil)
	if err != nil {
		panic(err) // we don't use options, this shouldn't happen
	}
	c.decoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(err) // we don't use options, this shouldn't happen
	}
	return c
}

// compressBytes compresses the input via zstd.
func (c compressor) compressBytes(src []byte) []byte {
	// ~50KiB of JSON containing base64 tends to compress to ~10% of its
	// original size. This size also seems like a good starting point for
	// most realistic compression ratios.
	estimate := len(src) / 10
	start := time.Now()
	dst := c.encoder.EncodeAll(src, make([]byte, 0, estimate))
	elapsed := time.Since(start)
	log.Debugf("compressed %.2f KiB to %.2f KiB in %s with zstd, %.1f%% of the original size",
		float64(len(src))/1000,
		float64(len(dst))/1000,
		elapsed,
		float64(len(dst)*100)/float64(len(src)))
	return dst
}

// isZstd reports whether the input bytes begin with zstd's magic number,
// 0xFD2FB528 in little-endian format.
//
// There are "magic number detection" modules, but most are pretty heavy and
// unnecessary, and we only need to detect zstd v1.
func isZstd(src []byte) bool {
	return len(src) >= 4 &&
		src[0] == 0x28 && src[1] == 0xB5 &&
		src[2] == 0x2f && src[3] == 0xFD
}

// decompressBytes tries to decompress the input as best it can. If it detects
// the input to be zstd, it decompresses using that algorithm. Otherwise, it
// assumes the input bytes aren't compressed and returns them as-is.
func (c compressor) decompressBytes(src []byte) []byte {
	if !isZstd(src) {
		// We assume that no compression is used, e.g. before we started
		// compressing census dumps when publishing to ipfs.
		return src
	}
	// We use a compressione stimate of 1/10th the size. Let's use 5x as a
	// starting point, following the same rule while being conservative.
	estimate := len(src) * 5
	start := time.Now()
	dst, err := c.decoder.DecodeAll(src, make([]byte, 0, estimate))
	if err != nil {
		log.Errorf("could not decompress zstd: %v", err)
		return nil
	}
	elapsed := time.Since(start)
	log.Debugf("decompressed %.2f KiB to %.2f KiB in %s with zstd, %.1f%% of the original size",
		float64(len(src))/1000,
		float64(len(dst))/1000,
		elapsed,
		float64(len(dst)*100)/float64(len(src)))
	return dst
}
