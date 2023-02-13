package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/compressor"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	CensusHandler = "censuses"

	CensusTypeWeighted   = "weighted"
	CensusTypeZKWeighted = "zkweighted"
	CensusTypeZK         = "zkindexed" // Will be deprecated soon
	CensusTypeCSP        = "csp"
	CensusTypeUnknown    = "unknown"

	MaxCensusAddBatchSize = 8192

	censusIDsize  = 32
	censusKeysize = 32
)

func (a *API) enableCensusHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/censuses/{type}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusCreateHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/participants",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusAddHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/type",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusTypeHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/root",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusRootHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/export",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusDumpHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/import",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusImportHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/weight",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusWeightHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/size",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusSizeHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/publish",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/publish/{root}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}",
		"DELETE",
		apirest.MethodAccessTypePublic,
		a.censusDeleteHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/proof/{key}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusProofHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/censuses/{censusID}/verify",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusVerifyHandler,
	); err != nil {
		return err
	}

	return nil
}

// POST /censuses/{type}
// create a new census
func (a *API) censusCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusType, indexed := censusType(ctx.URLParam("type"))
	if censusType == models.Census_UNKNOWN {
		return fmt.Errorf("census type is unknown")
	}

	maxLevels := a.vocapp.TransactionHandler.ZkCircuit.Config.Levels
	censusID := util.RandomBytes(32)
	_, err = a.censusdb.New(censusID, censusType, indexed, "", &token, maxLevels)
	if err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(Census{
		CensusID: censusID,
	}); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// POST /censuses/participants/{censusID}
// Adds one or multiple key/weights to the census
func (a *API) censusAddHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	cdata := CensusParticipants{}
	if err := json.Unmarshal(msg.Data, &cdata); err != nil {
		return err
	}
	if len(cdata.Participants) == 0 {
		return fmt.Errorf("missing participant parameters")
	}
	if len(cdata.Participants) > MaxCensusAddBatchSize {
		return fmt.Errorf("maximum number of participants per call is %d (received %d)",
			MaxCensusAddBatchSize, len(cdata.Participants))
	}

	ref, err := a.censusdb.Load(censusID, &token)
	if err != nil {
		return err
	}

	// build the list of keys and values that will be added to the three
	keys := [][]byte{}
	values := [][]byte{}
	for i, p := range cdata.Participants {
		if p.Key == nil {
			return fmt.Errorf("missing participant key number %d", i)
		}
		// check the weight parameter
		if p.Weight == nil {
			p.Weight = new(types.BigInt).SetUint64(1)
		}
		if ref.Indexed && p.Weight.MathBigInt().Uint64() != 1 {
			return fmt.Errorf("indexed census cannot use weight")
		}

		leafKey := p.Key
		if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
			// compute the hash, we use it as key for the merkle tree
			leafKey, err = ref.Tree().Hash(p.Key)
			if err != nil {
				return fmt.Errorf("could not compute key hash: %w", err)
			}
		}

		keys = append(keys, leafKey)
		if !ref.Indexed {
			values = append(values, ref.Tree().BigIntToBytes(p.Weight.MathBigInt()))
		}
	}

	// add the keys and values to the tree in a single transaction
	if !ref.Indexed {
		failed, err := ref.Tree().AddBatch(keys, values)
		if err != nil {
			return fmt.Errorf("cannot add key and value to tree: %w", err)
		}
		log.Infof("added %d keys to census %x", len(keys), censusID)
		if len(failed) > 0 {
			log.Warnf("failed participants %v", failed)
		}
	} else {
		failed, err := ref.Tree().AddBatch(keys, nil)
		if err != nil {
			return fmt.Errorf("cannot add key to tree: %w", err)
		}
		if len(failed) > 0 {
			log.Warnf("failed participants %v", failed)
		}
	}
	log.Infof("added %d keys to census %x", len(keys), censusID)

	return ctx.Send(nil, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/type
func (a *API) censusTypeHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	// Get the current census type from the disk
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}

	// Set unknown as default type and check other available types
	censusType := CensusTypeUnknown
	switch ref.CensusType {
	case int32(models.Census_ARBO_POSEIDON):
		censusType = CensusTypeZKWeighted
	case int32(models.Census_ARBO_BLAKE2B):
		censusType = CensusTypeWeighted
	case int32(models.Census_CA):
		censusType = CensusTypeCSP
	}

	// Envolves the type into the Census struct and return it as JSON
	data, err := json.Marshal(Census{Type: censusType})
	if err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/root
func (a *API) censusRootHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}
	root, err := ref.Tree().Root()
	if err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(Census{
		Root: root,
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/export
func (a *API) censusDumpHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, &token)
	if err != nil {
		return err
	}
	root, err := ref.Tree().Root()
	if err != nil {
		return err
	}
	dump, err := ref.Tree().Dump()
	if err != nil {
		return err
	}
	var data []byte
	if data, err = json.Marshal(censusdb.CensusDump{
		RootHash: root,
		Data:     compressor.NewCompressor().CompressBytes(dump),
		Type:     models.Census_Type(ref.CensusType),
		Indexed:  ref.Indexed,
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/import
func (a *API) censusImportHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	cdata := censusdb.CensusDump{}
	if err := json.Unmarshal(msg.Data, &cdata); err != nil {
		return err
	}
	if cdata.Data == nil || cdata.RootHash == nil {
		return fmt.Errorf("missing dump or root parameters")
	}

	ref, err := a.censusdb.Load(censusID, &token)
	if err != nil {
		return err
	}
	if ref.CensusType != int32(cdata.Type) {
		return fmt.Errorf("census type does not match")
	}
	if ref.Indexed != cdata.Indexed {
		return fmt.Errorf("indexed flag does not match")
	}

	if err := ref.Tree().ImportDump(compressor.NewCompressor().DecompressBytes(cdata.Data)); err != nil {
		return err
	}

	root, err := ref.Tree().Root()
	if err != nil {
		return err
	}

	if !bytes.Equal(root, cdata.RootHash) {
		return fmt.Errorf("root hash does not match after importing dump")
	}

	return ctx.Send(nil, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/weight
func (a *API) censusWeightHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}
	weight, err := ref.Tree().GetCensusWeight()
	if err != nil {
		return err
	}
	var data []byte
	if data, err = json.Marshal(Census{
		Weight: (*types.BigInt)(weight),
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/size
func (a *API) censusSizeHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}
	size, err := ref.Tree().Size()
	if err != nil {
		return err
	}
	var data []byte
	if data, err = json.Marshal(Census{
		Size: size,
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/delete
func (a *API) censusDeleteHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	_, err = a.censusdb.Load(censusID, &token)
	if err != nil {
		return err
	}
	if err := a.censusdb.Del(censusID); err != nil {
		return err
	}
	return ctx.Send(nil, apirest.HTTPstatusCodeOK)
}

// POST /censuses/{censusID}/publish/{root}
// POST /censuses/{censusID}/publish
func (a *API) censusPublishHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	ref, err := a.censusdb.Load(censusID, &token)
	if err != nil {
		return err
	}

	if root := ctx.URLParam("root"); root != "" {
		fromRoot, err := hex.DecodeString(root)
		if err != nil {
			return fmt.Errorf("could not decode root")
		}
		t, err := ref.Tree().FromRoot(fromRoot)
		if err != nil {
			return err
		}
		ref.SetTree(t)
	}

	// the root hash is used as censusID for the new published census
	// check if a census with censusID=root already exist
	root, err := ref.Tree().Root()
	if err != nil {
		return err
	}

	// if the census already exists, return the URI and the root
	if a.censusdb.Exists(root) {
		ref, err := a.censusdb.Load(root, nil)
		if err != nil {
			return err
		}
		var data []byte
		if data, err = json.Marshal(&Census{
			CensusID: root,
			URI:      ref.URI,
		}); err != nil {
			return err
		}
		return ctx.Send(data, apirest.HTTPstatusCodeOK)
	}

	// dump the current tree to import them after
	dump, err := ref.Tree().Dump()
	if err != nil {
		return err
	}

	// export the tree to the remote storage (IPFS)
	uri := ""
	if a.storage != nil {
		exportData, err := censusdb.BuildExportDump(
			root,
			dump,
			models.Census_Type(ref.CensusType),
			ref.Indexed,
			ref.MaxLevels,
		)
		if err != nil {
			return err
		}
		sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cid, err := a.storage.Publish(sctx, exportData)
		if err != nil {
			log.Errorf("could not export tree to storage: %v", err)
		} else {
			uri = a.storage.URIprefix() + cid
		}
	}

	newRef, err := a.censusdb.New(
		root, models.Census_Type(ref.CensusType),
		ref.Indexed, uri, nil, ref.MaxLevels)
	if err != nil {
		return err
	}
	if err := newRef.Tree().ImportDump(dump); err != nil {
		return err
	}
	newRef.Tree().Publish()

	var data []byte
	if data, err = json.Marshal(&Census{
		CensusID: root,
		URI:      uri,
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /censuses/{censusID}/proof/{key}
func (a *API) censusProofHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	key, err := censusKeyParse(ctx.URLParam("key"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}

	// If the census type is zkweighted (that means that it uses Poseidon hash
	// with Arbo merkle tree), skip to perform a hash function over the census
	// key. It is because the zk friendly key of any census leaf is the
	// ZkAddress which follows a specific transformation process that must be
	// implemented into the circom circuit also, and it is already hashed.
	// Otherwhise, hash the key before get the proof.
	leafKey := key
	if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
		leafKey, err = ref.Tree().Hash(key)
		if err != nil {
			return err
		}
	}

	leafV, siblings, err := ref.Tree().GenProof(leafKey)
	if err != nil {
		return err
	}
	response := Census{
		Proof: siblings,
		Value: leafV,
	}

	// Get the leaf siblings from arbo based on the key received and include
	// them into the response, only if it is zkweighted.
	if ref.CensusType == int32(models.Census_ARBO_POSEIDON) {
		response.Siblings, err = ref.Tree().GetCircomSiblings(leafKey)
		if err != nil {
			return err
		}
	}
	if len(leafV) > 0 && !ref.Tree().IsIndexed() {
		// return the string representation of the census value (weight)
		// to make the client know his voting power for the census
		weight := ref.Tree().BytesToBigInt(leafV)
		response.Weight = (*types.BigInt)(weight)
	}

	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// POST /censuses/{censusID}/verify
func (a *API) censusVerifyHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	cdata := Census{}
	if err := json.Unmarshal(msg.Data, &cdata); err != nil {
		return err
	}
	if cdata.Key == nil || cdata.Proof == nil {
		return fmt.Errorf("missing key or proof parameters")
	}

	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}

	// If the census type is zkweighted (that means that it uses Poseidon hash
	// with Arbo merkle tree), skip to perform a hash function over the census
	// key. It is because the zk friendly key of any census leaf is the
	// ZkAddress which follows a specific transformation process that must be
	// implemented into the circom circuit also, and it is already hashed.
	// Otherwhise, hash the key before verify the proof.
	leafKey := cdata.Key
	if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
		leafKey, err = ref.Tree().Hash(cdata.Key)
		if err != nil {
			return err
		}
	}

	valid, err := ref.Tree().VerifyProof(leafKey, cdata.Value, cdata.Proof, cdata.Root)
	if err != nil {
		return err
	}
	if !valid {
		return ctx.Send(nil, apirest.HTTPstatusCodeErr)
	}
	response := Census{
		Valid: valid,
	}
	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}
