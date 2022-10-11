package urlapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	CensusHandler = "census"

	censusDBprefix          = "cs_"
	censusDBreferencePrefix = "cr_"
	censusIDsize            = 32
	censusKeysize           = 32
)

type censusRef struct {
	tree       *censustree.Tree
	AuthToken  *uuid.UUID
	CensusType int32
	Indexed    bool
	IsPublic   bool
}

func (u *URLAPI) enableCensusHandlers() error {
	if err := u.api.RegisterMethod(
		"/census/create/{type}",
		"GET",
		bearerstdapi.MethodAccessTypePublic, // must be private in some moment
		u.censusCreateHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/add/{key}/{weight}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusAddHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/add/{key}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusAddHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/root",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusRootHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/dump",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusDumpHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/import",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		u.censusImportHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/weight",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusWeightHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/size",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusSizeHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/publish",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/publish/{root}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusPublishHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/delete",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusDeleteHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/proof/{key}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.censusProofHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/census/{censusID}/verify",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		u.censusVerifyHandler,
	); err != nil {
		return err
	}

	return nil
}

// /census/create/{type}
// create a new census
func (u *URLAPI) censusCreateHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusType, indexed := censusType(ctx.URLParam("type"))
	if censusType == models.Census_UNKNOWN {
		return fmt.Errorf("census type is unknown")
	}

	censusID := util.RandomBytes(32)
	_, err = u.createNewCensus(censusID, censusType, indexed, false, &token)
	if err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(Census{
		CensusID: censusID,
	}); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/add/{key}
// /census/{censusID}/add/{key}/{weight}
func (u *URLAPI) censusAddHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	key, err := censusKeyParse(ctx.URLParam("key"))
	if err != nil {
		return err
	}
	ref, err := u.loadCensus(censusID, &token)
	if err != nil {
		return err
	}
	var weight *types.BigInt
	if w := ctx.URLParam("weight"); w != "" {
		if ref.Indexed {
			return fmt.Errorf("indexed census cannot use weight")
		}
		weight, err = censusWeightParse(w)
		if err != nil {
			return err
		}
	} else if !ref.Indexed {
		weight.SetUint64(1)
	}
	keyHash, err := ref.tree.Hash(key)
	if err != nil {
		return err
	}
	if weight != nil {
		if err := ref.tree.Add(keyHash, ref.tree.BigIntToBytes(weight.ToInt())); err != nil {
			return fmt.Errorf("cannot add key and value to tree: %w", err)
		}
		log.Debugf("added key %x with weight %s to census %x", key, weight, censusID)
	} else {
		if err := ref.tree.Add(keyHash, nil); err != nil {
			return fmt.Errorf("cannot add key to tree: %w", err)
		}
		log.Debugf("added key %x to census %x", key, censusID)
	}
	return ctx.Send(nil, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/root
func (u *URLAPI) censusRootHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := u.loadCensus(censusID, nil)
	if err != nil {
		return err
	}
	root, err := ref.tree.Root()
	if err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(Census{
		Root: root,
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/dump
func (u *URLAPI) censusDumpHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := u.loadCensus(censusID, &token)
	if err != nil {
		return err
	}
	root, err := ref.tree.Root()
	if err != nil {
		return err
	}
	dump, err := ref.tree.Dump()
	if err != nil {
		return err
	}
	var data []byte
	if data, err = json.Marshal(CensusDump{
		RootHash: root,
		Data:     newCompressor().compressBytes(dump),
		Type:     models.Census_Type(ref.CensusType),
		Indexed:  ref.Indexed,
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/import
func (u *URLAPI) censusImportHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	cdata := CensusDump{}
	if err := json.Unmarshal(msg.Data, &cdata); err != nil {
		return err
	}
	if cdata.Data == nil || cdata.RootHash == nil {
		return fmt.Errorf("missing dump or root parameters")
	}

	ref, err := u.loadCensus(censusID, &token)
	if err != nil {
		return err
	}
	if ref.CensusType != int32(cdata.Type) {
		return fmt.Errorf("census type does not match")
	}
	if ref.Indexed != cdata.Indexed {
		return fmt.Errorf("indexed flag does not match")
	}

	if err := ref.tree.ImportDump(newCompressor().decompressBytes(cdata.Data)); err != nil {
		return err
	}

	root, err := ref.tree.Root()
	if err != nil {
		return err
	}

	if !bytes.Equal(root, cdata.RootHash) {
		return fmt.Errorf("root hash does not match after importing dump")
	}

	return ctx.Send(nil, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/weight
func (u *URLAPI) censusWeightHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := u.loadCensus(censusID, nil)
	if err != nil {
		return err
	}
	weight, err := ref.tree.GetCensusWeight()
	if err != nil {
		return err
	}
	var data []byte
	if data, err = json.Marshal(Census{
		Weight: (*types.BigInt)(weight),
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/size
func (u *URLAPI) censusSizeHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	ref, err := u.loadCensus(censusID, nil)
	if err != nil {
		return err
	}
	size, err := ref.tree.Size()
	if err != nil {
		return err
	}
	var data []byte
	if data, err = json.Marshal(Census{
		Size: size,
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/delete
func (u *URLAPI) censusDeleteHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	_, err = u.loadCensus(censusID, &token)
	if err != nil {
		return err
	}

	if err := u.delCensusRefFromDB(censusID); err != nil {
		return err
	}
	// return reply to the caller so the HTTP connection can be drop.
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	if err := ctx.Send(nil, bearerstdapi.HTTPstatusCodeOK); err != nil {
		log.Error(err)
	}
	_, err = censustree.DeleteCensusTreeFromDatabase(u.db, censusName(censusID))
	if err != nil {
		return err
	}
	return nil
}

// /census/{censusID}/publish/{root}
// /census/{censusID}/publish
func (u *URLAPI) censusPublishHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	token, err := uuid.Parse(msg.AuthToken)
	if err != nil {
		return err
	}
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}

	ref, err := u.loadCensus(censusID, &token)
	if err != nil {
		return err
	}

	if root := ctx.URLParam("root"); root != "" {
		fromRoot, err := hex.DecodeString(root)
		if err != nil {
			return fmt.Errorf("could not decode root")
		}
		ref.tree, err = ref.tree.FromRoot(fromRoot)
		if err != nil {
			return err
		}
	}

	// the root hash is used as censusID for the new published census
	// check if a census with censusID=root already exist
	root, err := ref.tree.Root()
	if err != nil {
		return err
	}

	if u.censusRefExist(root) {
		return fmt.Errorf("a published census with root %x already exist", root)
	}

	dump, err := ref.tree.Dump()
	if err != nil {
		return err
	}

	newRef, err := u.createNewCensus(
		root, models.Census_Type(ref.CensusType),
		ref.Indexed, true, nil)
	if err != nil {
		return err
	}
	if err := newRef.tree.ImportDump(dump); err != nil {
		return err
	}
	newRef.tree.Publish()

	// export the tree to the remote storage (IPFS)
	uri := ""
	if u.storage != nil {
		export := CensusDump{
			Type:     models.Census_Type(ref.CensusType),
			Indexed:  ref.Indexed,
			RootHash: root,
			Data:     newCompressor().compressBytes(dump),
		}
		exportData, err := json.Marshal(export)
		if err != nil {
			return err
		}

		sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cid, err := u.storage.Publish(sctx, exportData)
		if err != nil {
			log.Errorf("could not export tree to storage: %v", err)
		} else {
			uri = u.storage.URIprefix() + cid
		}
	}

	var data []byte
	if data, err = json.Marshal(&Census{
		CensusID: root,
		URI:      uri,
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /census/{censusID}/proof/{key}
func (u *URLAPI) censusProofHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	censusID, err := censusIDparse(ctx.URLParam("censusID"))
	if err != nil {
		return err
	}
	key, err := censusKeyParse(ctx.URLParam("key"))
	if err != nil {
		return err
	}
	ref, err := u.loadCensus(censusID, nil)
	if err != nil {
		return err
	}
	keyHash, err := ref.tree.Hash(key)
	if err != nil {
		return err
	}
	leafV, siblings, err := ref.tree.GenProof(keyHash)
	if err != nil {
		return err
	}

	response := Census{
		Proof: siblings,
		Value: leafV,
	}
	if len(leafV) > 0 && !ref.tree.IsIndexed() {
		// return the string representation of the census value (weight)
		// to make the client know his voting power for the census
		weight := ref.tree.BytesToBigInt(leafV)
		response.Weight = (*types.BigInt)(weight)
	}

	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// POST /census/{censusID}/verify
func (u *URLAPI) censusVerifyHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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

	ref, err := u.loadCensus(censusID, nil)
	if err != nil {
		return err
	}
	keyHash, err := ref.tree.Hash(cdata.Key)
	if err != nil {
		return err
	}
	valid, err := ref.tree.VerifyProof(keyHash, cdata.Value, cdata.Proof, cdata.Root)
	if err != nil {
		return err
	}
	if !valid {
		return ctx.Send(nil, bearerstdapi.HTTPstatusCodeErr)
	}
	response := Census{
		Valid: valid,
	}
	var data []byte
	if data, err = json.Marshal(&response); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
