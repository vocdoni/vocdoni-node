package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/censustree"
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
	CensusTypeCSP        = "csp"
	CensusTypeUnknown    = "unknown"

	MaxCensusAddBatchSize = 8192

	censusIDsize = 32
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

// censusCreateHandler
//
//	@Summary		Create a new census
//	@Description	Create a new census
//	@Success		200	{object}	Census
//	@Router			/censuses/{type} [post]
func (a *API) censusCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusType := decodeCensusType(ctx.URLParam("type"))
	if censusType == models.Census_UNKNOWN {
		return ErrCensusTypeUnknown
	}

	maxLevels := a.vocapp.TransactionHandler.ZkCircuit.Config.Levels
	censusID := util.RandomBytes(32)
	_, err = a.censusdb.New(censusID, censusType, "", &token, maxLevels)
	if err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(Census{
		CensusID: censusID,
	}); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusAddHandler
//
//	@Summary		Add participants to census
//	@Description	Adds one or multiple key/weights to the census
//	@Success		200	"(empty body)"
//	@Router			/censuses/{censusID}/participants [post]
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
		return ErrParamParticipantsTooBig.Withf("expected %d, got %d", MaxCensusAddBatchSize, len(cdata.Participants))
	}

	ref, err := a.censusdb.Load(censusID, &token)
	if err != nil {
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
		return err
	}
	// Instance variable to send any deprecation message
	// TODO: design a feture to standarize this kind of behaviour for future
	// uses cases
	var deprecationMsg []byte

	// build the list of keys and values that will be added to the tree
	keys := [][]byte{}
	values := [][]byte{}
	for i, p := range cdata.Participants {
		if p.Key == nil {
			return ErrParticipantKeyMissing.Withf("number %d", i)
		}
		// check the weight parameter
		// TODO: (lucasmenendez) remove that check, now all census are weighted
		if p.Weight == nil {
			p.Weight = new(types.BigInt).SetUint64(1)
		}

		leafKey := p.Key
		if len(leafKey) > censustree.DefaultMaxKeyLen {
			deprecationMsg = []byte(fmt.Sprintf("census keys must not be longer "+
				"than %d, the key provided has been truncated to this length",
				censustree.DefaultMaxKeyLen))
		}

		if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
			// compute the hash, we use it as key for the merkle tree
			leafKey, err = ref.Tree().Hash(p.Key)
			if err != nil {
				return ErrCantComputeKeyHash.WithErr(err)
			}
			// If the provided key is longer than the defined maximum length
			// truncate it.
			// TODO: return an error if the other key lengths are deprecated
			if len(leafKey) > censustree.DefaultMaxKeyLen {
				leafKey = leafKey[:censustree.DefaultMaxKeyLen]
			}
		}

		keys = append(keys, leafKey)
		values = append(values, ref.Tree().BigIntToBytes(p.Weight.MathBigInt()))
	}

	// add the keys and values to the tree in a single transaction
	failed, err := ref.Tree().AddBatch(keys, values)
	if err != nil {
		return ErrCantAddKeyAndValueToTree.WithErr(err)
	}
	log.Infof("added %d keys to census %x", len(keys), censusID)
	if len(failed) > 0 {
		log.Warnf("failed participants %v", failed)
	}

	return ctx.Send(deprecationMsg, apirest.HTTPstatusOK)
}

// censusTypeHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/type [get]
func (a *API) censusTypeHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	// Get the current census type from the disk
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
		return err
	}
	// Encode the current censusType to string
	censusType := encodeCensusType(models.Census_Type(ref.CensusType))
	// Envolves the type into the Census struct and return it as JSON
	data, err := json.Marshal(Census{Type: censusType})
	if err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusRootHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/root [get]
func (a *API) censusRootHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusDumpHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	censusdb.CensusDump
//	@Router			/censuses/{censusID}/export [get]
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
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
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
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusImportHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	"(empty body)"
//	@Router			/censuses/{censusID}/import [post]
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
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
		return err
	}
	if ref.CensusType != int32(cdata.Type) {
		return ErrCensusTypeMismatch
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

	return ctx.Send(nil, apirest.HTTPstatusOK)
}

// censusWeightHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/weight [get]
func (a *API) censusWeightHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusSizeHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/size [get]
func (a *API) censusSizeHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	if err != nil {
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusDeleteHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	"(empty body)"
//	@Router			/censuses/{censusID} [delete]
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
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
		return err
	}
	if err := a.censusdb.Del(censusID); err != nil {
		return err
	}
	return ctx.Send(nil, apirest.HTTPstatusOK)
}

// censusPublishHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/publish [post]
//	@Router			/censuses/{censusID}/publish/{root} [post]
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
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
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
			if errors.Is(err, censusdb.ErrCensusNotFound) {
				return ErrCensusNotFound
			}
			return err
		}
		var data []byte
		if data, err = json.Marshal(&Census{
			CensusID: root,
			URI:      ref.URI,
		}); err != nil {
			return err
		}
		return ctx.Send(data, apirest.HTTPstatusOK)
	}

	// dump the current tree to import them after
	dump, err := ref.Tree().Dump()
	if err != nil {
		return err
	}

	// export the tree to the remote storage (IPFS)
	uri := ""
	if a.storage != nil {
		exportData, err := censusdb.BuildExportDump(root, dump,
			models.Census_Type(ref.CensusType), ref.MaxLevels)
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

	newRef, err := a.censusdb.New(root, models.Census_Type(ref.CensusType), uri,
		nil, ref.MaxLevels)
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusProofHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/proof/{key} [get]
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
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
		return err
	}
	// Get the census type to return it into the response to prevent to perform
	// two api calls.
	censusType := encodeCensusType(models.Census_Type(ref.CensusType))
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
		return fmt.Errorf("something was wrong during census proof generation: %w", err)
	}
	response := Census{
		Proof: siblings,
		Value: leafV,
		Type:  censusType,
	}

	// Get the leaf siblings from arbo based on the key received and include
	// them into the response, only if it is zkweighted.
	if ref.CensusType == int32(models.Census_ARBO_POSEIDON) {
		response.Siblings, err = ref.Tree().GetCircomSiblings(leafKey)
		if err != nil {
			return fmt.Errorf("error geting the census siblings to circom: %w", err)
		}
	}
	if len(leafV) > 0 {
		// return the string representation of the census value (weight)
		// to make the client know his voting power for the census
		weight := ref.Tree().BytesToBigInt(leafV)
		response.Weight = (*types.BigInt)(weight)
	}

	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusVerifyHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	Census
//	@Router			/censuses/{censusID}/verify [post]
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
		if errors.Is(err, censusdb.ErrCensusNotFound) {
			return ErrCensusNotFound
		}
		return err
	}

	// If the census type is zkweighted (that means that it uses Poseidon hash
	// with Arbo merkle tree), skip to perform a hash function over the census
	// key. It is because the zk friendly key of any census leaf is the
	// ZkAddress which follows a specific transformation process that must be
	// implemented into the circom circuit also, and it is already hashed.
	// Otherwhise, hash the key before get the proof.
	leafKey := cdata.Key
	if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
		leafKey, err = ref.Tree().Hash(cdata.Key)
		if err != nil {
			return err
		}
	}

	valid, err := ref.Tree().VerifyProof(leafKey, cdata.Value, cdata.Proof, cdata.Root)
	if err != nil {
		return fmt.Errorf("something was wrong during census proof verification: %w", err)
	}
	if !valid {
		return ctx.Send(nil, apirest.HTTPstatusBadRequest)
	}
	response := Census{
		Valid: valid,
	}
	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
