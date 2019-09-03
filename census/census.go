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

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	tree "gitlab.com/vocdoni/go-dvote/tree"
)

type CensusNamespaces struct {
	RootKey    string      `json:"rootKey"` //Public key allowed to created new census
	Namespaces []Namespace `json:"namespaces"`
}

type Namespace struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

type CensusManager struct {
	Storage    string                //Root storage data dir
	AuthWindow int32                 //Time window (seconds) in which TimeStamp will be accepted if auth enabled
	Census     CensusNamespaces      //Available namespaces
	Trees      map[string]*tree.Tree //MkTrees map of merkle trees indexed by censusId
	Data       *data.Storage
}

// Init creates a new census manager
func (cm *CensusManager) Init(storage, rootKey string) error {
	nsConfig := fmt.Sprintf("%s/namespaces.json", storage)
	cm.Storage = storage
	cm.Trees = make(map[string]*tree.Tree)
	cm.AuthWindow = 10

	log.Infof("loading namespaces and keys from %s", nsConfig)
	if _, err := os.Stat(nsConfig); os.IsNotExist(err) {
		log.Info("creating new config file")
		var cns CensusNamespaces
		if len(rootKey) < signature.PubKeyLength {
			//log.Warn("no root key provided or invalid, anyone will be able to create new census")
			cns.RootKey = ""
		} else {
			cns.RootKey = rootKey
		}
		cm.Census = cns
		ioutil.WriteFile(nsConfig, []byte(""), 0644)
		err = cm.save()
		return err
	}

	jsonFile, err := os.Open(nsConfig)
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &cm.Census)
	if err != nil {
		log.Warn("could not unmarshal json config file, probably empty. Skipping")
		return nil
	}
	if len(rootKey) >= signature.PubKeyLength {
		log.Infof("updating root key to %s", rootKey)
		cm.Census.RootKey = rootKey
	} else {
		log.Infof("current root key %s", rootKey)
	}
	// Initialize existing merkle trees
	for _, ns := range cm.Census.Namespaces {
		t := tree.Tree{}
		t.Storage = cm.Storage
		err := t.Init(ns.Name)
		if err != nil {
			log.Warn(err.Error())
		} else {
			log.Infof("initialized merkle tree %s", ns.Name)
			cm.Trees[ns.Name] = &t
		}
	}
	return nil
}

// AddNamespace adds a new merkletree identified by a censusId (name)
func (cm *CensusManager) AddNamespace(name string, pubKeys []string) error {
	log.Infof("adding namespace %s", name)
	if _, e := cm.Trees[name]; e {
		return errors.New("namespace already exist")
	}
	mkTree := tree.Tree{}
	mkTree.Storage = cm.Storage
	err := mkTree.Init(name)
	if err != nil {
		return err
	}
	cm.Trees[name] = &mkTree
	var ns Namespace
	ns.Name = name
	ns.Keys = pubKeys
	cm.Census.Namespaces = append(cm.Census.Namespaces, ns)
	return cm.save()
}

func (cm *CensusManager) save() error {
	log.Info("saving namespaces")
	nsConfig := fmt.Sprintf("%s/namespaces.json", cm.Storage)
	data, err := json.Marshal(cm.Census)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(nsConfig, data, 644)
}

func httpReply(resp *types.CensusResponseMessage, w http.ResponseWriter) {
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
func (cm *CensusManager) CheckAuth(crm *types.MessageRequest) error {
	if len(crm.Signature) < signature.SignatureLength || len(crm.Request.CensusID) < 1 {
		return errors.New("signature or censusId not provided or invalid")
	}
	ns := new(Namespace)
	for _, n := range cm.Census.Namespaces {
		if n.Name == crm.Request.CensusID {
			ns = &n
		}
	}

	// Add root key, if method is addCensus
	if crm.Request.Method == "addCensus" {
		if len(cm.Census.RootKey) < signature.PubKeyLength {
			log.Warn("root key does not exist, considering addCensus valid for any request")
			return nil
		}
		ns.Keys = []string{cm.Census.RootKey}
	}

	if ns == nil {
		return errors.New("censusId not valid")
	}

	// Check timestamp
	currentTime := int32(time.Now().Unix())
	if crm.Request.Timestamp > currentTime+cm.AuthWindow ||
		crm.Request.Timestamp < currentTime-cm.AuthWindow {
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
		msg, err := json.Marshal(crm.Request)
		if err != nil {
			return errors.New("cannot unmarshal")
		}
		for _, n := range ns.Keys {
			valid, err = signature.Verify(string(msg), crm.Signature, n)
			if err != nil {
				log.Warnf("verification error (%s)", err.Error())
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
func (cm *CensusManager) HTTPhandler(w http.ResponseWriter, req *http.Request, signer *signature.SignKeys) {
	log.Debug("new request received")
	var rm types.MessageRequest
	if ok := checkRequest(w, req); !ok {
		return
	}
	// Decode JSON
	log.Debug("decoding JSON")
	err := json.NewDecoder(req.Body).Decode(&rm)
	if err != nil {
		log.Warnf("cannot decode JSON: %s", err.Error())
		http.Error(w, err.Error(), 400)
		return
	}
	if len(rm.Request.Method) < 1 {
		http.Error(w, "method must be specified", 400)
		return
	}
	log.Debugf("found method %s", rm.Request.Method)
	auth := true
	err = cm.CheckAuth(&rm)
	if err != nil {
		log.Warnf("authorization error: %s", err.Error())
		auth = false
	}
	resp := cm.Handler(&rm.Request, auth, "")
	respMsg := new(types.CensusResponseMessage)
	respMsg.Response = *resp
	respMsg.ID = rm.ID
	respMsg.Response.Request = rm.ID
	respMsg.Signature, err = signer.SignJSON(respMsg.Response)
	if err != nil {
		log.Warn(err.Error())
	}
	httpReply(respMsg, w)
}

// Handler handles an API census manager request.
// isAuth gives access to the private methods only if censusPrefix match or censusPrefix not defined
// censusPrefix should usually be the Ethereum Address or a Hash of the allowed PubKey
func (cm *CensusManager) Handler(r *types.MetaRequest, isAuth bool, censusPrefix string) *types.CensusResponse {
	resp := new(types.CensusResponse)
	op := r.Method
	var err error

	// Process data
	log.Infof("processing data %+v", *r)
	resp.Ok = true
	resp.Error = ""
	resp.Timestamp = int32(time.Now().Unix())

	if op == "addCensus" {
		if isAuth {
			err = cm.AddNamespace(censusPrefix+r.CensusID, r.PubKeys)
			if err != nil {
				log.Warnf("error creating census: %s", err.Error())
				resp.Ok = false
				resp.Error = err.Error()
			} else {
				log.Infof("census %s%s created successfully managed by %s", censusPrefix, r.CensusID, r.PubKeys)
				resp.CensusID = censusPrefix + r.CensusID
			}
		} else {
			resp.Ok = false
			resp.Error = "invalid authentication"
		}
		return resp
	}

	censusFound := false
	for k := range cm.Trees {
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

	// validAuthPrefix is true: either censusPrefix is not used or censusID contains the prefix
	validAuthPrefix := false
	if len(censusPrefix) == 0 {
		validAuthPrefix = true
		log.Debugf("prefix not specified, allowing access to all census IDs if pubkey validation correct")
	} else {
		validAuthPrefix = strings.HasPrefix(r.CensusID, censusPrefix)
		log.Debugf("prefix allowed for %s", r.CensusID)
	}

	//Methods without rootHash
	if op == "getRoot" {
		resp.Root = cm.Trees[r.CensusID].GetRoot()
		return resp
	}

	if op == "addClaimBulk" {
		if isAuth && validAuthPrefix {
			addedClaims := 0
			var invalidClaims []int
			for i, c := range r.ClaimsData {
				data, err := base64.StdEncoding.DecodeString(c)
				if err == nil {
					err = cm.Trees[r.CensusID].AddClaim(data)
				}
				if err != nil {
					log.Warnf("error adding claim: %s", err.Error())
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
			resp.Ok = false
			resp.Error = "invalid authentication"
		}
		return resp
	}

	if op == "addClaim" {
		if isAuth && validAuthPrefix {
			data, err := base64.StdEncoding.DecodeString(r.ClaimData)
			if err != nil {
				log.Warnf("error decoding base64 string: %s", err.Error())
				resp.Ok = false
				resp.Error = err.Error()
			}
			err = cm.Trees[r.CensusID].AddClaim(data)
			if err != nil {
				log.Warnf("error adding claim: %s", err.Error())
				resp.Ok = false
				resp.Error = err.Error()
			} else {
				log.Debugf("claim added %x", data)
			}
		} else {
			resp.Ok = false
			resp.Error = "invalid authentication"
		}
		return resp
	}

	if op == "importDump" {
		if isAuth && validAuthPrefix {
			if len(r.ClaimsData) > 0 {
				err = cm.Trees[r.CensusID].ImportDump(r.ClaimsData)
				if err != nil {
					log.Warnf("error importing dump: %s", err.Error())
					resp.Ok = false
					resp.Error = err.Error()
				} else {
					log.Infof("dump imported successfully, %d claims", len(r.ClaimsData))
				}
			}
		} else {
			resp.Ok = false
			resp.Error = "invalid authentication"
		}
		return resp
	}

	if op == "importRemote" {
		// To-Do implement Gzip compression
		if !isAuth || !validAuthPrefix {
			resp.Ok = false
			resp.Error = "invalid authentication"
			return resp
		}
		if cm.Data == nil {
			resp.Ok = false
			resp.Error = "not supported"
			return resp
		}
		dataStorage := *cm.Data
		if !strings.HasPrefix(r.URI, dataStorage.GetURIprefix()) ||
			len(r.URI) <= len(dataStorage.GetURIprefix()) {
			log.Warnf("uri not supported %s (supported prefix %s)", r.URI, dataStorage.GetURIprefix())
			resp.Ok = false
			resp.Error = "URI not supported"
			return resp
		}
		log.Infof("retrieving remote census %s", r.CensusURI)
		censusRaw, err := dataStorage.Retrieve(r.URI[len(dataStorage.GetURIprefix()):])
		if err != nil {
			log.Warnf("cannot retrieve census: %s", err.Error())
			resp.Ok = false
			resp.Error = "cannot retrieve census"
			return resp
		}
		var dump types.CensusDump
		err = json.Unmarshal(censusRaw, &dump)
		if err != nil {
			log.Warnf("retrieved census do not have a correct format: %s", err.Error())
			resp.Ok = false
			resp.Error = "retrieved census do not have a correct format"
			return resp
		}
		log.Infof("retrieved census with rootHash %s and size %d bytes", dump.RootHash, len(censusRaw))
		if len(dump.ClaimsData) > 0 {
			err = cm.Trees[r.CensusID].ImportDump(dump.ClaimsData)
			if err != nil {
				log.Warnf("error importing dump: %s", err.Error())
				resp.Ok = false
				resp.Error = "error importing census"
			} else {
				log.Infof("dump imported successfully, %d claims", len(dump.ClaimsData))
			}
		} else {
			log.Warnf("no claims found on the retreived census")
			resp.Ok = false
			resp.Error = "no claims found"
		}
		return resp
	}

	//Methods with rootHash, if rootHash specified snapshot the tree
	var t *tree.Tree
	if len(r.RootHash) > 1 { //if rootHash specified
		t, err = cm.Trees[r.CensusID].Snapshot(r.RootHash)
		if err != nil {
			log.Warnf("snapshot error: %s", err.Error())
			resp.Ok = false
			resp.Error = "invalid root hash"
			return resp
		}
	} else { //if rootHash not specified use current tree
		t = cm.Trees[r.CensusID]
	}

	if op == "genProof" {
		resp.Siblings, err = t.GenProof([]byte(r.ClaimData))
		if err != nil {
			resp.Ok = false
			resp.Error = err.Error()
		}
		return resp
	}

	if op == "dump" || op == "dumpPlain" {
		if !isAuth || !validAuthPrefix {
			resp.Ok = false
			resp.Error = "invalid authentication"
			return resp
		}
		//dump the claim data and return it
		var dumpValues []string
		if op == "dump" {
			dumpValues, err = t.Dump(r.RootHash)
		} else {
			dumpValues, err = t.DumpPlain(r.RootHash)
		}
		if err != nil {
			resp.Error = err.Error()
			resp.Ok = false
		} else {

			resp.ClaimsData = dumpValues
		}
		return resp
	}

	if op == "publish" {
		// To-Do implement Gzip compression
		if !isAuth || !validAuthPrefix {
			resp.Ok = false
			resp.Error = "invalid authentication"
			return resp
		}
		if cm.Data == nil {
			resp.Ok = false
			resp.Error = "not supported"
			return resp
		}
		var dump types.CensusDump
		dump.RootHash = t.GetRoot()
		dump.ClaimsData, err = t.Dump(t.GetRoot())
		if err != nil {
			resp.Error = err.Error()
			resp.Ok = false
			log.Warnf("cannot dump census with root %s: %s", t.GetRoot(), err.Error())
			return resp
		}
		dumpBytes, err := json.Marshal(dump)
		if err != nil {
			resp.Error = err.Error()
			resp.Ok = false
			log.Warnf("cannot marshal census dump: %s", err.Error())
			return resp
		}
		dataStorage := *cm.Data
		cid, err := dataStorage.Publish(dumpBytes)
		resp.URI = dataStorage.GetURIprefix() + cid
		log.Infof("published census at %s", resp.URI)
		resp.Root = t.GetRoot()
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
