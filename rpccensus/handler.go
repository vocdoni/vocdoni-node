package rpccensus

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	api "go.vocdoni.io/dvote/rpctypes"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

// Handler handles an RPC census manager request.
// This function will be deprecated soon.
func (m *Manager) Handler(ctx context.Context, r *api.APIrequest,
	censusPrefix string) (*api.APIresponse, error) {
	resp := new(api.APIresponse)

	// Process data
	log.Debugf("processing data %s", r.String())
	resp.Ok = true
	resp.Timestamp = int32(time.Now().Unix())

	censusID, err := hex.DecodeString(util.TrimHex(r.CensusID))
	if err != nil {
		// if censusID is not hex, then hash it
		censusID = ethereum.HashRaw([]byte(r.CensusID))
	}
	// Special methods not depending on census existence
	if r.Method == "addCensus" {
		if r.CensusType == models.Census_UNKNOWN {
			r.CensusType = models.Census_ARBO_BLAKE2B
		}
		if r.CensusType != models.Census_ARBO_BLAKE2B && r.CensusType != models.Census_ARBO_POSEIDON {
			return nil, fmt.Errorf("census type not supported: %s", r.CensusType)
		}
		if _, err := m.cdb.New(
			censusID,
			r.CensusType,
			r.CensusType == models.Census_ARBO_POSEIDON,
			"",
			nil,
			160,
		); err != nil {
			return nil, err
		}
		log.Infof("census %s created, successfully", r.CensusID)
		resp.CensusID = r.CensusID
		return resp, nil
	}

	// check if census exist
	if !m.cdb.Exists(censusID) {
		return nil, fmt.Errorf("censusId not valid or not found %s", r.CensusID)
	}

	// Load the merkle tree
	ref, err := m.cdb.Load(censusID, nil)
	if err != nil {
		return nil, fmt.Errorf("censusId cannot be loaded")
	}

	// Methods without rootHash
	switch r.Method {
	case "getRoot":
		resp.Root, err = ref.Tree().Root()
		if err != nil {
			return nil, err
		}
		return resp, nil

	case "addClaimBulk":
		if len(r.Weights) > 0 && len(r.Weights) != len(r.CensusKeys) {
			return nil, fmt.Errorf("weights and censusKeys have different size")
		}
		var batchKeys [][]byte
		if !r.Digested {
			batchKeys = [][]byte{}
			for _, k := range r.CensusKeys {
				hk, err := ref.Tree().Hash(k)
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
				batchValues = append(batchValues, ref.Tree().BigIntToBytes(v.MathBigInt()))
			}
		} else {
			// If no weights specified, assume al weight values are equal to 1
			for i := 0; i < len(r.CensusKeys); i++ {
				batchValues = append(batchValues, ref.Tree().BigIntToBytes(big.NewInt(1)))
			}
		}
		invalidClaims, err := ref.Tree().AddBatch(batchKeys, batchValues)
		if err != nil {
			return nil, err
		}
		if len(invalidClaims) > 0 {
			resp.InvalidClaims = invalidClaims
		}
		resp.Root, err = ref.Tree().Root()
		if err != nil {
			return nil, err
		}
		log.Infof("%d claims added successfully", len(r.CensusKeys)-len(invalidClaims))
		return resp, nil

	case "addClaim":
		if r.CensusKey == nil {
			return resp, fmt.Errorf("error decoding claim data")
		}
		key := r.CensusKey
		if !r.Digested {
			if key, err = ref.Tree().Hash(key); err != nil {
				return resp, fmt.Errorf("error digesting data: %w", err)
			}
		}
		value := ref.Tree().BigIntToBytes(big.NewInt(1))
		if r.Weight != nil {
			value = ref.Tree().BigIntToBytes(r.Weight.MathBigInt())
		}
		err := ref.Tree().Add(key, value)
		if err != nil {
			return nil, err
		}
		resp.Root, err = ref.Tree().Root()
		if err != nil {
			return nil, err
		}
		log.Debugf("claim added (key:%x value:%x)", key, value)
		return resp, nil

	case "checkProof":
		if len(r.ProofData) < 1 {
			return resp, fmt.Errorf("proofData not provided")
		}
		var err error
		var root []byte
		if len(r.RootHash) < 1 {
			root, err = ref.Tree().Root()
			if err != nil {
				return nil, err
			}
		} else {
			root = r.RootHash
		}
		// Generate proof and return it
		key := r.CensusKey
		if !r.Digested {
			if key, err = ref.Tree().Hash(key); err != nil {
				return resp, fmt.Errorf("error digesting data: %w", err)
			}
		}
		// For legacy compatibility, assume weight=1 if value is nil
		if r.CensusValue == nil {
			r.CensusValue = ref.Tree().BigIntToBytes(big.NewInt(1))
		}
		validProof, err := ref.Tree().VerifyProof(key, r.CensusValue, r.ProofData, root)
		if err != nil {
			return nil, err
		}
		resp.ValidProof = &validProof
		return resp, nil
	}

	switch r.Method {
	case "genProof":
		key := r.CensusKey
		if !r.Digested {
			if key, err = ref.Tree().Hash(key); err != nil {
				return nil, fmt.Errorf("error digesting data: %w", err)
			}
		}
		resp.CensusValue, resp.Siblings, err = ref.Tree().GenProof(key)
		if err != nil {
			return nil, err
		}
		// Weight is 1 by default
		resp.Weight = new(types.BigInt).SetUint64(1)

		if ref.Indexed {
			// When the census is indexed thus the key is not the user's
			// public key from the request, we prefix it to the proof so that
			// the user receives the real proof key as well.
			//
			// This is legacy compatibility for the old API. The new censustree API
			// handles this internally.
			index, err := ref.Tree().KeyToIndex(key)
			if err != nil {
				return nil, err
			}
			resp.Siblings = append(index, resp.Siblings...)
		}

		if len(resp.CensusValue) > 0 && !ref.Indexed {
			// return also the string representation of the census value (weight)
			// to make the client know his voting power for the census
			weight := ref.Tree().BytesToBigInt(resp.CensusValue)
			resp.Weight = (*types.BigInt)(weight)
		}
		return resp, nil

	case "getSize":
		size, err := ref.Tree().Size()
		if err != nil {
			return nil, err
		}
		sizeInt64 := int64(size)
		resp.Size = &sizeInt64
		return resp, nil

	case "getCensusWeight":
		weight, err := ref.Tree().GetCensusWeight()
		if err != nil {
			return nil, err
		}
		resp.Weight = (*types.BigInt)(weight)
		return resp, nil

	case "dumpPlain":
		return nil, fmt.Errorf("dumpPlain is deprecated, dump should be used instead")

	case "dump":
		// dump the claim data and return it
		var root []byte
		if len(r.RootHash) < 1 {
			root, err = ref.Tree().Root()
			if err != nil {
				return nil, err
			}
		} else {
			root = r.RootHash
		}
		snapshot, err := ref.Tree().FromRoot(root)
		if err != nil {
			return nil, err
		}
		resp.CensusDump, err = snapshot.Dump()
		if err != nil {
			return nil, err
		}
		return resp, nil

	case "publish":
		dump, err := ref.Tree().Dump()
		if err != nil {
			return nil, err
		}
		root, err := ref.Tree().Root()
		if err != nil {
			return nil, err
		}
		newRef, err := m.cdb.New(
			root, models.Census_Type(ref.CensusType),
			ref.Indexed, "", nil, ref.MaxLevels)
		if err != nil {
			return nil, err
		}
		if err := newRef.Tree().ImportDump(dump); err != nil {
			return nil, err
		}
		newRef.Tree().Publish()

		// export the tree to the remote storage (IPFS)
		uri := ""
		if m.RemoteStorage != nil {
			exportData, err := censusdb.BuildExportDump(
				root,
				dump,
				models.Census_Type(ref.CensusType),
				ref.Indexed,
				ref.MaxLevels,
			)
			if err != nil {
				return nil, err
			}
			sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cid, err := m.RemoteStorage.Publish(sctx, exportData)
			if err != nil {
				log.Errorf("could not export tree to storage: %v", err)
			} else {
				uri = m.RemoteStorage.URIprefix() + cid
			}
		}

		log.Infof("published census %x at %s", root, uri)
		resp.URI = uri
		resp.Root = root
	}
	return resp, nil
}
