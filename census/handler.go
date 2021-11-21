package census

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

type CensusDump struct {
	Type     models.Census_Type `json:"type"`
	RootHash []byte             `json:"rootHash"`
	Data     []byte             `json:"data"`
}

// Handler handles an API census manager request.
// isAuth gives access to the private methods only if censusPrefix match or censusPrefix not defined
// censusPrefix should usually be the Ethereum Address or a Hash of the allowed PubKey
func (m *Manager) Handler(ctx context.Context, r *api.APIrequest,
	censusPrefix string) (*api.APIresponse, error) {
	resp := new(api.APIresponse)

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
		if r.CensusType == models.Census_UNKNOWN {
			r.CensusType = models.Census_ARBO_BLAKE2B
		}
		if r.CensusType != models.Census_ARBO_BLAKE2B && r.CensusType != models.Census_ARBO_POSEIDON {
			return nil, fmt.Errorf("census type not supported: %s", r.CensusType)
		}
		t, err := m.AddNamespace(censusPrefix+r.CensusID, r.CensusType, r.PubKeys)
		if err != nil {
			return nil, err
		}
		t.Publish()
		log.Infof("census %s%s created, successfully managed by %s", censusPrefix, r.CensusID, r.PubKeys)
		resp.CensusID = censusPrefix + r.CensusID
		return resp, nil
	}

	if r.Method == "getCensusList" {
		for n := range m.Trees {
			resp.CensusList = append(resp.CensusList, n)
		}
		return resp, nil
	}

	// check if census exist
	m.TreesMu.RLock()
	exists := m.Exists(r.CensusID)
	m.TreesMu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("censusId not valid or not found %s", r.CensusID)
	}

	// validAuthPrefix is true: either censusPrefix is not used or censusID contains the prefix
	validAuthPrefix := false
	if len(censusPrefix) == 0 {
		validAuthPrefix = true
		log.Debugf("prefix not specified, allowing access to all census IDs if pubkey validation is correct")
	} else {
		validAuthPrefix = strings.HasPrefix(r.CensusID, censusPrefix)
		log.Debugf("prefix allowed for %s", r.CensusID)
	}

	// Load the merkle tree
	m.TreesMu.Lock()
	tr, ok := m.Trees[r.CensusID]
	m.TreesMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("censusId cannot be loaded")
	}
	if !tr.IsPublic() {
		return resp, fmt.Errorf("census not yet published")
	}

	var err error
	// Methods without rootHash
	switch r.Method {
	case "getRoot":
		resp.Root, err = tr.Root()
		if err != nil {
			return nil, err
		}
		return resp, nil

	case "addClaimBulk":
		if validAuthPrefix {
			if len(r.Weights) > 0 && len(r.Weights) != len(r.CensusKeys) {
				return nil, fmt.Errorf("weights and censusKeys have different size")
			}
			var batchKeys [][]byte
			if !r.Digested {
				batchKeys = [][]byte{}
				for _, k := range r.CensusKeys {
					hk, err := tr.Hash(k)
					if err != nil {
						return nil, fmt.Errorf("error hashig key: %w", err)
					}
					batchKeys = append(batchKeys, hk)
				}
			} else {
				batchKeys = r.CensusKeys
			}
			var batchValues [][]byte
			if len(r.Weights) > 0 {
				for _, v := range r.Weights {
					batchValues = append(batchValues, tr.BigIntToBytes(v.ToInt()))
				}
			} else {
				// If no weights specified, assume al weight values are equal to 1
				for i := 0; i < len(r.CensusKeys); i++ {
					batchValues = append(batchValues, tr.BigIntToBytes(big.NewInt(1)))
				}
			}
			invalidClaims, err := tr.AddBatch(batchKeys, batchValues)
			if err != nil {
				return nil, err
			}
			if len(invalidClaims) > 0 {
				resp.InvalidClaims = invalidClaims
			}
			resp.Root, err = tr.Root()
			if err != nil {
				return nil, err
			}
			log.Infof("%d claims addedd successfully", len(r.CensusKeys)-len(invalidClaims))
		} else {
			return nil, fmt.Errorf("invalid authentication")
		}
		return resp, nil

	case "addClaim":
		if validAuthPrefix {
			if r.CensusKey == nil {
				return resp, fmt.Errorf("error decoding claim data")
			}
			key := r.CensusKey
			if !r.Digested {
				if key, err = tr.Hash(key); err != nil {
					return resp, fmt.Errorf("error digesting data: %w", err)
				}
			}
			value := tr.BigIntToBytes(big.NewInt(1))
			if r.Weight != nil {
				value = tr.BigIntToBytes(r.Weight.ToInt())
			}
			err := tr.Add(key, value)
			if err != nil {
				return nil, err
			}
			resp.Root, err = tr.Root()
			if err != nil {
				return nil, err
			}
			log.Debugf("claim added (key:%x value:%x)", key, value)
		} else {
			return nil, fmt.Errorf("invalid authentication")
		}
		return resp, nil

	case "importDump":
		if validAuthPrefix {
			if len(r.CensusKeys) > 0 {
				// FIXME: Can cause Txn too big
				err := tr.ImportDump(r.CensusDump)
				if err != nil {
					log.Warnf("error importing dump: %s", err)
					return nil, err
				}
				log.Infof("dump imported successfully, %d claims", len(r.CensusKeys))
			}
		} else {
			return nil, fmt.Errorf("invalid authentication")
		}
		return resp, nil

	case "importRemote":
		if !validAuthPrefix {
			return nil, fmt.Errorf("invalid authentication")
		}
		if m.RemoteStorage == nil {
			return nil, fmt.Errorf("import remote storage not supported")
		}
		if !strings.HasPrefix(r.URI, m.RemoteStorage.URIprefix()) ||
			len(r.URI) <= len(m.RemoteStorage.URIprefix()) {
			return nil, fmt.Errorf("URI not supported")
		}
		log.Infof("retrieving remote census %s", r.CensusURI)
		censusRaw, err := m.RemoteStorage.Retrieve(ctx, r.URI[len(m.RemoteStorage.URIprefix()):], 0)
		if err != nil {
			log.Warnf("cannot retrieve census: %s", err)
			return nil, fmt.Errorf("cannot retrieve census")
		}
		censusRaw = m.decompressBytes(censusRaw)
		var dump CensusDump
		err = json.Unmarshal(censusRaw, &dump)
		if err != nil {
			log.Warnf("retrieved census do not have a correct format: %s", err)
			return nil, fmt.Errorf("retrieved census do not have a correct format")
		}
		log.Infof("retrieved census with rootHash %x and size %d bytes", dump.RootHash, len(censusRaw))
		if len(dump.Data) > 0 {
			err = tr.ImportDump(dump.Data)
			if err != nil {
				log.Warnf("error importing dump: %s", err)
				return nil, fmt.Errorf("error importing census")
			}
			log.Infof("dump imported successfully, %d bytes", len(dump.Data))
		} else {
			log.Warnf("no data found on the retreived census")
			return nil, fmt.Errorf("no claims found")
		}
		return resp, nil

	case "checkProof":
		if len(r.ProofData) < 1 {
			return resp, fmt.Errorf("proofData not provided")
		}
		var err error
		var root []byte
		if len(r.RootHash) < 1 {
			root, err = tr.Root()
			if err != nil {
				return nil, err
			}
		} else {
			root = r.RootHash
		}
		// Generate proof and return it
		key := r.CensusKey
		if !r.Digested {
			if key, err = tr.Hash(key); err != nil {
				return resp, fmt.Errorf("error digesting data: %w", err)
			}
		}
		// For legacy compatibility, assume weight=1 if value is nil
		if r.CensusValue == nil {
			r.CensusValue = tr.BigIntToBytes(big.NewInt(1))
		}
		validProof, err := tr.VerifyProof(key, r.CensusValue, r.ProofData, root)
		if err != nil {
			return nil, err
		}
		resp.ValidProof = &validProof
		return resp, nil
	}

	// Methods with rootHash, if rootHash specified snapshot the tree.
	// Otherwise, we use the same tree.
	if len(r.RootHash) > 1 {
		var err error
		tr, err = tr.FromRoot(r.RootHash)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch snapshot for root")
		}
	}

	switch r.Method {
	case "genProof":
		key := r.CensusKey
		if !r.Digested {
			if key, err = tr.Hash(key); err != nil {
				return nil, fmt.Errorf("error digesting data: %w", err)
			}
		}
		// If tr.Type() == ARBO_POSEIDON, resolve mapping key -> index before genProof
		var proofPrefix []byte
		if tr.Type() == models.Census_ARBO_POSEIDON {
			// key is 64 bits index, little endian encoded
			key, err = m.KeyToIndex(r.CensusID, key)
			if err != nil {
				return nil, err
			}
			proofPrefix = key
		}
		leafV, siblings, err := tr.GenProof(key)
		if err != nil {
			return nil, err
		}
		// When the key is not the user's public key from the request,
		// we prefix it to the proof so that the user receives the real
		// proof key as well.
		if len(proofPrefix) != 0 {
			siblings = append(proofPrefix, siblings...)
		}
		resp.Siblings = siblings

		if len(leafV) > 0 {
			resp.CensusValue = leafV
			// return also the string representation of the census value (weight)
			// to make the client know his voting power for the census
			weight := tr.BytesToBigInt(leafV)
			resp.Weight = (*types.BigInt)(weight)
		}
		return resp, nil

	case "getSize":
		size, err := tr.Size()
		if err != nil {
			return nil, err
		}
		sizeInt64 := int64(size)
		resp.Size = &sizeInt64
		return resp, nil

	case "dumpPlain":
		return nil, fmt.Errorf("dumpPlain is deprecated, dump should be used instead")

	case "dump":
		if !validAuthPrefix {
			return resp, fmt.Errorf("invalid authentication")
		}
		// dump the claim data and return it
		var root []byte
		if len(r.RootHash) < 1 {
			root, err = tr.Root()
			if err != nil {
				return nil, err
			}
		} else {
			root = r.RootHash
		}
		snapshot, err := tr.FromRoot(root)
		if err != nil {
			return nil, err
		}
		resp.CensusDump, err = snapshot.Dump()
		if err != nil {
			return nil, err
		}
		return resp, nil

	case "publish":
		if !validAuthPrefix {
			return nil, fmt.Errorf("invalid authentication")
		}
		if m.RemoteStorage == nil {
			return nil, fmt.Errorf("not supported")
		}
		var dump CensusDump

		root, err := tr.Root()
		if err != nil {
			return nil, err
		}
		dump.RootHash = root
		snapshot, err := tr.FromRoot(root)
		if err != nil {
			log.Warnf("cannot do a tree from root (in order to do a census dump) with root %x: %s", root, err)
			return nil, err
		}
		dump.Data, err = snapshot.Dump()
		if err != nil {
			log.Warnf("cannot dump census with root %x: %s", root, err)
			return nil, err
		}
		dump.Type = tr.Type()
		dumpBytes, err := json.Marshal(dump)
		if err != nil {
			log.Warnf("cannot marshal census dump: %s", err)
			return nil, err
		}
		dumpBytes = m.compressBytes(dumpBytes)
		cid, err := m.RemoteStorage.Publish(ctx, dumpBytes)
		if err != nil {
			log.Warnf("cannot publish census dump: %s", err)
			return nil, err
		}
		resp.URI = m.RemoteStorage.URIprefix() + cid
		log.Infof("published census at %s", resp.URI)
		resp.Root = root

		// adding published census with censusID = rootHash
		log.Infof("adding new namespace for published census %x", resp.Root)
		namespace := hex.EncodeToString(resp.Root)
		tr2, err := m.AddNamespace(namespace, tr.Type(), r.PubKeys)
		if err != nil && err != ErrNamespaceExist {
			log.Warnf("error creating local published census: %s", err)
		} else if err == nil {
			log.Infof("import claims to new census")
			err = tr2.ImportDump(dump.Data)
			if err != nil {
				_ = m.DelNamespace(namespace)
				log.Warn(err)
				return nil, err
			}
			tr2.Publish()
		}
	}
	return resp, nil
}
