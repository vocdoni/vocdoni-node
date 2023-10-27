package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
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
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{type}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusCreateHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/participants",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusAddHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/type",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusTypeHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/root",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusRootHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/export",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusDumpHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/import",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusImportHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/weight",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusWeightHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/size",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusSizeHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/publish",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/publish/{root}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}",
		"DELETE",
		apirest.MethodAccessTypePublic,
		a.censusDeleteHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/proof/{key}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusProofHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusID}/verify",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusVerifyHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/export",
		"GET",
		apirest.MethodAccessTypeAdmin,
		a.censusExportDBHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/import",
		"POST",
		apirest.MethodAccessTypeAdmin,
		a.censusImportDBHandler,
	); err != nil {
		return err
	}

	return nil
}

// censusCreateHandler
//
//	@Summary				Create a new census
//	@Description.markdown	censusCreateHandler
//	@Tags					Censuses
//	@Accept					json
//	@Produce				json
//	@Security				BasicAuth
//	@Param					type	path		string	true	"Census type"	Enums(weighted,zkweighted,csp)
//	@Success				200		{object}	object{censusId=string}
//	@Router					/censuses/{type} [post]
func (a *API) censusCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusType := decodeCensusType(ctx.URLParam("type"))
	if censusType == models.Census_UNKNOWN {
		return ErrCensusTypeUnknown
	}

	// get census max levels from vochain app if available
	maxLevels := circuit.CircuitsConfigurations[circuit.DefaultCircuitConfigurationTag].Levels
	if a.vocapp != nil {
		maxLevels = a.vocapp.TransactionHandler.ZkCircuit.Config.Levels
	}

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
//	@Summary				Add participants to census
//	@Description.markdown	censusAddHandler
//	@Tags					Censuses
//	@Accept					json
//	@Produce				json
//	@Security				BasicAuth
//	@Param					censusID	path	string				true	"Census id"
//	@Param					transaction	body	CensusParticipants	true	"PublicKey - weight array "
//	@Success				200			"(empty body)"
//	@Router					/censuses/{censusID}/participants [post]
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
			return ErrInvalidCensusKeyLength.Withf("the census key cannot be longer than %d bytes", censustree.DefaultMaxKeyLen)
		}

		if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
			// compute the hash, we use it as key for the merkle tree
			leafKey, err = ref.Tree().Hash(p.Key)
			if err != nil {
				return ErrCantComputeKeyHash.WithErr(err)
			}
			leafKey = leafKey[:censustree.DefaultMaxKeyLen]
		}

		keys = append(keys, leafKey)
		values = append(values, ref.Tree().BigIntToBytes(p.Weight.MathBigInt()))
	}

	// add the keys and values to the tree in a single transaction
	log.Infow("adding participants to census", "census", hex.EncodeToString(censusID), "count", len(keys))
	failed, err := ref.Tree().AddBatch(keys, values)
	if err != nil {
		return ErrCantAddKeyAndValueToTree.WithErr(err)
	}
	if len(failed) > 0 {
		log.Warnw("failed to add some participants to census",
			"census", hex.EncodeToString(censusID), "count", len(failed))
	}

	return ctx.Send(deprecationMsg, apirest.HTTPstatusOK)
}

// censusTypeHandler
//
//	@Summary		Get type of census
//	@Description	Get the census type
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			censusID	path		string					true	"Census id"
//	@Success		200			{object}	object{census=string}	"Census type "weighted", "zkweighted", "csp"
//	@Router			/censuses/{censusID}/type [get]
func (a *API) censusTypeHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		Census Merkle Root
//	@Description	Get census [Merkle Tree root](https://docs.vocdoni.io/architecture/census/off-chain-tree.html) hash, used to identify the census at specific snapshot.\n\n- Bearer token not required
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			censusID	path		string				true	"Census id"
//	@Success		200			{object}	object{root=string}	"Merkle root of the census"
//	@Router			/censuses/{censusID}/root [get]
func (a *API) censusRootHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
		CensusRoot: root,
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusDumpHandler
//
//	@Summary		Export census
//	@Description	Export census to JSON format. Requires Bearer token
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Security		BasicAuth
//	@Param			censusID	path		string	true	"Census id"
//	@Success		200			{object}	censusdb.CensusDump
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
//	@Summary		Import census
//	@Description	Import census from JSON previously exported using [`/censuses/{censusId}/export`](census-export). Requires Bearer token
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Security		BasicAuth
//	@Param			censusID	path	string	true	"Census id"
//	@Success		200			"(empty body)"
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
//	@Summary		Census total weight
//	@Description	It sums all weights added to the census. Weight is a stringified bigInt
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			censusID	path		string					true	"Census id"
//	@Success		200			{object}	object{weight=string}	"Sum of weight son a stringfied big int format"
//	@Router			/censuses/{censusID}/weight [get]
func (a *API) censusWeightHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		Census size
//	@Description	Total number of keys added to the census. Size as integer
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			censusID	path		string				true	"Census id"
//	@Success		200			{object}	object{size=string}	"Size as integer"
//	@Router			/censuses/{censusID}/size [get]
func (a *API) censusSizeHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		Delete census
//	@Description	Delete unpublished census (not on the storage yet). See [publish census](census-publish)\n
//	@Description	- Requires Bearer token
//	@Description	- Deletes a census from the server storage
//	@Description	- Published census cannot be deleted
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			censusID	path	string	true	"Census id"
//	@Success		200			"(empty body)"
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
//	@Summary				Publish census
//	@Description.markdown	censusPublishHandler
//	@Tags					Censuses
//	@Accept					json
//	@Produce				json
//	@Security				BasicAuth
//	@Success				200			{object}	object{census=object{censusID=string,uri=string}}	"It return published censusID and the ipfs uri where its uploaded"
//	@Param					censusID	path		string												true	"Census id"
//	@Router					/censuses/{censusID}/publish [post]
//	/censuses/{censusID}/publish/{root} [post] Endpoint docs generated on docs/models/model.go
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
//	@Summary				Prove key to census
//	@Description.markdown	censusProofHandler
//	@Tags					Censuses
//	@Accept					json
//	@Produce				json
//	@Security				BasicAuth
//	@Param					censusID	path		string											true	"Census id"
//	@Param					key			path		string											true	"Key to proof"
//	@Success				200			{object}	object{weight=number,proof=string,value=string}	"where proof is Merkle tree siblings and value is Merkle tree leaf value"
//	@Router					/censuses/{censusID}/proof/{key} [get]
func (a *API) censusProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	address, err := censusKeyParse(ctx.URLParam("key"))
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
	leafKey := address
	if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
		leafKey, err = ref.Tree().Hash(address)
		if err != nil {
			return err
		}
	}
	response := Census{Type: censusType}
	response.Value, response.CensusProof, err = ref.Tree().GenProof(leafKey)
	if err != nil {
		return ErrKeyNotFoundInCensus.With(hex.EncodeToString(leafKey))
	}
	if response.CensusRoot, err = ref.Tree().Root(); err != nil {
		return ErrCensusRootIsNil.WithErr(err)
	}

	// Get the leaf siblings from arbo based on the key received and include
	// them into the response, only if it is zkweighted.
	if ref.CensusType == int32(models.Census_ARBO_POSEIDON) {
		response.CensusSiblings, err = zk.ProofToCircomSiblings(response.CensusProof)
		if err != nil {
			return ErrCantGetCircomSiblings.WithErr(err)
		}
	}
	if len(response.Value) > 0 {
		// return the string representation of the census value (weight)
		// to make the client know his voting power for the census
		weight := ref.Tree().BytesToBigInt(response.Value)
		response.Weight = (*types.BigInt)(weight)
	}
	// encode response
	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusVerifyHandler
//
//	@Summary		Verify merkle proof
//	@Description	Verify that a previously obtained Merkle proof for a key, acquired via [/censuses/{censusId}/proof/{publicKey}](prove-key-to-census) is still correct.
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			censusID	path		string	true	"Census id"
//	@Success		200			{object}	object{valid=bool}
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
	if cdata.Key == nil || cdata.CensusProof == nil {
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
	// Otherwise, hash the key before get the proof.
	leafKey := cdata.Key
	if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
		leafKey, err = ref.Tree().Hash(cdata.Key)
		if err != nil {
			return err
		}
	}

	valid, err := ref.Tree().VerifyProof(leafKey, cdata.Value, cdata.CensusProof, cdata.CensusRoot)
	if err != nil {
		return ErrCensusProofVerificationFailed.WithErr(err)
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

// censusExportDBHandler
//
//	@Summary		Export census database
//	@Description	Export the whole census database to a JSON file. Requires Admin Bearer token.
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{valid=bool}
//	@Router			/censuses/export [get]
func (a *API) censusExportDBHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	buf := bytes.Buffer{}
	if err := a.censusdb.ExportCensusDB(&buf); err != nil {
		return err
	}
	return ctx.Send(buf.Bytes(), apirest.HTTPstatusOK)
}

// censusImportHandler
//
//	@Summary		Import census database
//	@Description	Import the whole census database from a JSON file.
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{valid=bool}
//	@Router			/censuses/import [post]
func (a *API) censusImportDBHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	if err := a.censusdb.ImportCensusDB(bytes.NewReader(msg.Data)); err != nil {
		return err
	}
	return ctx.Send(nil, apirest.HTTPstatusOK)
}
