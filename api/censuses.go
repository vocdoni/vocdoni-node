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
		return ErrCensusTypeUnknown
	}

	censusID := util.RandomBytes(32)
	_, err = a.censusdb.New(censusID, censusType, indexed, "", &token)
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
		return ErrParamParticipantsMissing
	}
	if len(cdata.Participants) > MaxCensusAddBatchSize {
		return fmt.Errorf("%w (%d, got %d)", ErrParamParticipantsTooBig,
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
			return fmt.Errorf("%w (number %d)", ErrParticipantKeyMissing, i)
		}
		// check the weight parameter
		if p.Weight == nil {
			p.Weight = new(types.BigInt).SetUint64(1)
		}
		if ref.Indexed && p.Weight.MathBigInt().Uint64() != 1 {
			return ErrIndexedCensusCantUseWeight
		}
		// compute the hash, we use it as key for the merkle tree
		keyHash, err := ref.Tree().Hash(p.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCantComputeKeyHash, err)
		}
		keys = append(keys, keyHash)
		if !ref.Indexed {
			values = append(values, ref.Tree().BigIntToBytes(p.Weight.MathBigInt()))
		}
	}

	// add the keys and values to the tree in a single transaction
	if !ref.Indexed {
		failed, err := ref.Tree().AddBatch(keys, values)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCantAddKeyAndValueToTree, err)
		}
		log.Infof("added %d keys to census %x", len(keys), censusID)
		if len(failed) > 0 {
			log.Warnf("failed participants %v", failed)
		}
	} else {
		failed, err := ref.Tree().AddBatch(keys, nil)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCantAddKeyToTree, err)
		}
		if len(failed) > 0 {
			log.Warnf("failed participants %v", failed)
		}
	}
	log.Infof("added %d keys to census %x", len(keys), censusID)

	return ctx.Send(nil, apirest.HTTPstatusCodeOK)
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
		return ErrParamDumpOrRootMissing
	}

	ref, err := a.censusdb.Load(censusID, &token)
	if err != nil {
		return err
	}
	if ref.CensusType != int32(cdata.Type) {
		return ErrCensusTypeMismatch
	}
	if ref.Indexed != cdata.Indexed {
		return ErrCensusIndexedFlagMismatch
	}

	if err := ref.Tree().ImportDump(compressor.NewCompressor().DecompressBytes(cdata.Data)); err != nil {
		return err
	}

	root, err := ref.Tree().Root()
	if err != nil {
		return err
	}

	if !bytes.Equal(root, cdata.RootHash) {
		return ErrCensusRootHashMismatch
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
			return ErrParamRootInvalid
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
		ref.Indexed, uri, nil)
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
	keyHash, err := ref.Tree().Hash(key)
	if err != nil {
		return err
	}
	leafV, siblings, err := ref.Tree().GenProof(keyHash)
	if err != nil {
		return err
	}

	response := Census{
		Proof: siblings,
		Value: leafV,
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
		return ErrParamKeyOrProofMissing
	}

	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		return err
	}
	keyHash, err := ref.Tree().Hash(cdata.Key)
	if err != nil {
		return err
	}
	valid, err := ref.Tree().VerifyProof(keyHash, cdata.Value, cdata.Proof, cdata.Root)
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
