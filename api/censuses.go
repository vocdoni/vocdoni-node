package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	CensusTypeFarcaster  = "farcaster"
	CensusTypeUnknown    = "unknown"

	MaxCensusAddBatchSize = 8192

	censusIDsize          = 32
	censusRetrieveTimeout = 10 * time.Minute
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
		"/censuses/{censusId}/participants",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusAddHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/type",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusTypeHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/root",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusRootHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/export",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusDumpHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/import",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusImportHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/weight",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusWeightHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/size",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusSizeHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/publish",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/publish/async",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/check",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusPublishCheckHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/publish/{root}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}",
		"DELETE",
		apirest.MethodAccessTypePublic,
		a.censusDeleteHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/proof/{key}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.censusProofHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/{censusId}/verify",
		"POST",
		apirest.MethodAccessTypePublic,
		a.censusVerifyHandler,
	); err != nil {
		return err
	}

	// Admin-only endpoints
	if err := a.Endpoint.RegisterMethod(
		"/censuses/export/ipfs",
		"GET",
		apirest.MethodAccessTypeAdmin,
		a.censusExportDBHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/censuses/export/ipfs/list",
		"GET",
		apirest.MethodAccessTypeAdmin,
		a.censusExportIPFSListDBHandler,
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
		"/censuses/import/{ipfscid}",
		"GET",
		apirest.MethodAccessTypeAdmin,
		a.censusImportDBHandler,
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
	if err := a.Endpoint.RegisterMethod(
		"/censuses/list",
		"GET",
		apirest.MethodAccessTypeAdmin,
		a.censusListHandler,
	); err != nil {
		return err
	}

	// Initialize the map to store the status of the async export to ipfs
	censusIPFSExports = make(map[string]time.Time)

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
	censusType := decodeCensusType(ctx.URLParam(ParamType))
	if censusType == models.Census_UNKNOWN {
		return ErrCensusTypeUnknown
	}

	// census max levels is limited by global ZkCircuit Levels
	censusID := util.RandomBytes(32)
	_, err = a.censusdb.New(censusID, censusType, "", &token, circuit.Global().Config.Levels)
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
//	@Param					censusId	path	string				true	"Census id"
//	@Param					transaction	body	CensusParticipants	true	"PublicKey - weight array "
//	@Success				200			"(empty body)"
//	@Router					/censuses/{censusId}/participants [post]
func (a *API) censusAddHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
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
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path		string					true	"Census id"
//	@Success		200			{object}	object{census=string}	"Census type "weighted", "zkweighted", "csp"
//	@Router			/censuses/{censusId}/type [get]
func (a *API) censusTypeHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	// Get the current census type from the disk
	ref, err := a.censusdb.Load(censusID, nil)
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path		string				true	"Census id"
//	@Success		200			{object}	object{root=string}	"Merkle root of the census"
//	@Router			/censuses/{censusId}/root [get]
func (a *API) censusRootHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path		string	true	"Census id"
//	@Success		200			{object}	censusdb.CensusDump
//	@Router			/censuses/{censusId}/export [get]
func (a *API) censusDumpHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, &token)
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path	string	true	"Census id"
//	@Success		200			"(empty body)"
//	@Router			/censuses/{censusId}/import [post]
func (a *API) censusImportHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
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
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path		string					true	"Census id"
//	@Success		200			{object}	object{weight=string}	"Sum of weight son a stringfied big int format"
//	@Router			/censuses/{censusId}/weight [get]
func (a *API) censusWeightHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path		string				true	"Census id"
//	@Success		200			{object}	object{size=string}	"Size as integer"
//	@Router			/censuses/{censusId}/size [get]
func (a *API) censusSizeHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path	string	true	"Census id"
//	@Success		200			"(empty body)"
//	@Router			/censuses/{censusId} [delete]
func (a *API) censusDeleteHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	_, err = a.censusdb.Load(censusID, &token)
	defer a.censusdb.UnLoad()
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
//	@Param					censusId	path		string												true	"Census id"
//	@Param					root		path		string												false	"Specific root where to publish the census. Not required"
//	@Router					/censuses/{censusId}/publish [post]
//	@Router					/censuses/{censusId}/publish/async [post]
//	@Router					/censuses/{censusId}/publish/{root} [post]
func (a *API) censusPublishHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}

	// check if the request is async
	url := strings.Split(ctx.Request.URL.Path, "/")
	async := url[len(url)-1] == "async"

	ref, err := a.censusdb.Load(censusID, &token)
	defer a.censusdb.UnLoad()
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
		a.censusdb.UnLoad()
		// note that there is already a UnLoad call set on defer
		ref, err := a.censusdb.Load(root, nil)
		if err != nil {
			if errors.Is(err, censusdb.ErrCensusNotFound) {
				return ErrCensusNotFound
			}
			return err
		}
		// if async, store the URI in the map for the check endpoint
		if async {
			a.censusPublishStatusMap.Store(hex.EncodeToString(root), ref.URI)
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

	publishCensus := func() (string, error) {
		// dump the current tree to import them after
		dump, err := ref.Tree().Dump()
		if err != nil {
			return "", err
		}

		// export the tree to the remote storage (IPFS)
		uri := ""
		if a.storage != nil {
			exportData, err := censusdb.BuildExportDump(root, dump,
				models.Census_Type(ref.CensusType), ref.MaxLevels)
			if err != nil {
				return "", err
			}
			sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			return "", err
		}
		return uri, newRef.Tree().ImportDump(dump)
	}

	if async {
		a.censusPublishStatusMap.Store(hex.EncodeToString(root), "")

		go func() {
			uri, err := publishCensus()
			if err != nil {
				log.Errorw(err, "could not publish census")
				a.censusPublishStatusMap.Store(hex.EncodeToString(root), fmt.Sprintf("error: %v", err.Error()))
				return
			}
			a.censusPublishStatusMap.Store(hex.EncodeToString(root), uri)
		}()

		var data []byte
		if data, err = json.Marshal(&Census{
			CensusID: root,
		}); err != nil {
			return err
		}

		return ctx.Send(data, apirest.HTTPstatusOK)
	}

	uri, err := publishCensus()
	if err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(&Census{
		CensusID: root,
		URI:      uri,
	}); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusPublishCheckHandler
//
//	@Summary				Check census publish status
//	@Description.markdown	censusPublishCheckHandler
//	@Tags					Censuses
//	@Produce				json
//	@Success				200			{object}	object{census=object{censusID=string,uri=string}}	"It return published censusID and the ipfs uri where its uploaded"
//	@Param					censusId	path		string												true	"Census id"
//	@Router					/censuses/{censusId}/check [get]
func (a *API) censusPublishCheckHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	uriOrErrorAny, ok := a.censusPublishStatusMap.Load(hex.EncodeToString(censusID))
	if !ok {
		return ErrCensusNotFound
	}
	uriOrError := uriOrErrorAny.(string)
	if uriOrError == "" {
		return ctx.Send(nil, apirest.HTTPstatusNoContent)
	}
	if strings.HasPrefix(uriOrError, "error:") {
		return ErrCensusBuild.With(uriOrError[7:])
	}
	var data []byte
	if data, err = json.Marshal(&Census{
		CensusID: censusID,
		URI:      uriOrError,
	}); err != nil {
		return err
	}
	a.censusPublishStatusMap.Delete(hex.EncodeToString(censusID))
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
//	@Param					censusId	path		string											true	"Census id"
//	@Param					key			path		string											true	"Key to proof"
//	@Success				200			{object}	object{weight=number,proof=string,value=string}	"where proof is Merkle tree siblings and value is Merkle tree leaf value"
//	@Router					/censuses/{censusId}/proof/{key} [get]
func (a *API) censusProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
	if err != nil {
		return err
	}
	address, err := censusKeyParse(ctx.URLParam("key"))
	if err != nil {
		return err
	}
	ref, err := a.censusdb.Load(censusID, nil)
	defer a.censusdb.UnLoad()
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
//	@Param			censusId	path		string	true	"Census id"
//	@Success		200			{object}	object{valid=bool}
//	@Router			/censuses/{censusId}/verify [post]
func (a *API) censusVerifyHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
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
	defer a.censusdb.UnLoad()
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

	if err := ref.Tree().VerifyProof(leafKey, cdata.Value, cdata.CensusProof, cdata.CensusRoot); err != nil {
		if strings.Contains(err.Error(), "calculated vs expected root mismatch") {
			return ctx.Send(nil, apirest.HTTPstatusBadRequest)
		}
		return ErrCensusProofVerificationFailed.WithErr(err)
	}

	response := Census{
		Valid: true,
	}
	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusListHandler
//
//	@Summary		List all census references
//	@Description	List all census references. Requires Admin Bearer token.
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{valid=bool}
//	@Router			/censuses/list [get]
func (a *API) censusListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	list, err := a.censusdb.List()
	if err != nil {
		return err
	}
	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusIPFSExports is a map of ipfs uri to the time when the export was requested
var censusIPFSExports = map[string]time.Time{}

// censusExportDBHandler
//
//	@Summary		Export census database
//	@Description	Export the whole census database to a JSON file. Requires Admin Bearer token.
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Param			ipfs	path		string	true	"Export to IPFS. Blank to return the JSON file"
//	@Success		200		{object}	object{valid=bool}
//	@Router			/censuses/export/ipfs [get]
//	@Router			/censuses/export [get]
func (a *API) censusExportDBHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	isIPFSExport := strings.HasSuffix(ctx.Request.URL.Path, "ipfs")
	buf := bytes.Buffer{}
	var data []byte
	if isIPFSExport {
		go func() {
			log.Infow("exporting census database to ipfs async")
			startTime := time.Now()
			if err := a.censusdb.ExportCensusDB(&buf); err != nil {
				log.Errorw(err, "could not export census database")
				return
			}
			log.Infow("census database exported", "duration (s)", time.Since(startTime).Seconds())
			startTime = time.Now()
			uri, err := a.storage.PublishReader(context.Background(), &buf)
			if err != nil {
				log.Errorw(err, "could not publish census database to ipfs")
				return
			}
			log.Infow("census database published to ipfs", "uri", uri, "duration (s)", time.Since(startTime).Seconds())
			censusIPFSExports[uri] = time.Now()
		}()
		var err error
		data, err = json.Marshal(map[string]string{
			"message": "scheduled, check /censuses/export/ipfs/list",
		})
		if err != nil {
			log.Errorw(err, "could not marshal response")
		}
	} else {
		if err := a.censusdb.ExportCensusDB(&buf); err != nil {
			return err
		}
		data = buf.Bytes()
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusExportIPFSListDBHandler
//
//	@Summary		List export census database to IPFS
//	@Description	List the IPFS URIs of the census database exports
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{valid=bool}
//	@Router			/censuses/export/ipfs/list [get]
func (a *API) censusExportIPFSListDBHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	data, err := json.Marshal(censusIPFSExports)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// censusImportHandler
//
//	@Summary		Import census database
//	@Description	Import the whole census database from a JSON file.
//	@Tags			Censuses
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{valid=bool}
//	@Router			/censuses/import/{ipfscid} [get]
//	@Router			/censuses/import [post]
func (a *API) censusImportDBHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	ipfscid := ctx.URLParam("ipfscid")
	if ipfscid == "" {
		// if no ipfs cid, import from the request body
		if err := a.censusdb.ImportCensusDB(bytes.NewReader(msg.Data)); err != nil {
			return err
		}
		return ctx.Send(nil, apirest.HTTPstatusOK)
	}
	log.Infow("importing census database from ipfs async", "ipfscid", ipfscid)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), censusRetrieveTimeout)
		defer cancel()
		// TODO: Retrieve should return a reader
		startTime := time.Now()
		data, err := a.storage.Retrieve(ctx, "/ipfs/"+ipfscid, 0)
		if err != nil {
			log.Errorw(err, "could not retrieve census database from ipfs")
			return
		}
		log.Infow("census database retrieved from ipfs", "ipfscid", ipfscid, "duration", time.Since(startTime))
		if err := a.censusdb.ImportCensusDB(bytes.NewReader(data)); err != nil {
			log.Errorw(err, "could not import census database from ipfs")
		}
	}()
	return ctx.Send(nil, apirest.HTTPstatusOK)
}
