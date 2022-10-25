package api

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

var censusDBlock = sync.RWMutex{}

func censusType(t string) (models.Census_Type, bool) {
	switch t {
	case CensusTypeZK:
		return models.Census_ARBO_POSEIDON, true
	case CensusTypeWeighted:
		return models.Census_ARBO_BLAKE2B, false
	}
	return models.Census_UNKNOWN, false
}

func censusIDparse(censusID string) ([]byte, error) {
	censusID = util.TrimHex(censusID)
	if len(censusID) != censusIDsize*2 {
		return nil, fmt.Errorf("invalid censusID format")
	}
	return hex.DecodeString(censusID)
}

func censusKeyParse(key string) ([]byte, error) {
	key = util.TrimHex(key)
	return hex.DecodeString(key)
}

func censusWeightParse(w string) (*types.BigInt, error) {
	weight, err := new(types.BigInt).SetString(w)
	if err != nil {
		return nil, err
	}
	return weight, nil
}

func censusName(censusID []byte) string {
	return fmt.Sprintf("%s%x", censusDBprefix, censusID)
}

func (a *API) createNewCensus(censusID []byte, censusType models.Census_Type,
	indexed, public bool, authToken *uuid.UUID) (*censusRef, error) {
	tree, err := censustree.New(censustree.Options{Name: censusName(censusID), ParentDB: a.db,
		MaxLevels: 256, CensusType: censusType, IndexAsKeysCensus: indexed})
	if err != nil {
		return nil, err
	}
	ref, err := a.addCensusRefToDB(censusID, authToken, censusType, indexed, public)
	if err != nil {
		return nil, err
	}
	ref.tree = tree
	if public {
		ref.tree.Publish()
	}
	// add the tree in memory so we can quickly access to it afterwards
	a.censusMap.Store(string(censusID), *ref)
	return ref, nil
}

func (a *API) censusRefExist(censusID []byte) bool {
	_, ok := a.censusMap.Load(string(censusID))
	return ok
}

// loadCensus returns an already loaded census from memory or from the persistent kv database.
// Authentication is checked if authToken is not nil.
func (a *API) loadCensus(censusID []byte, authToken *uuid.UUID) (*censusRef, error) {
	// if the tree is in memory, just return it
	val, ok := a.censusMap.Load(string(censusID))
	if ok {
		ref, refOk := val.(censusRef)
		if !refOk {
			panic("stored value is not of censusRef type")
		}
		if authToken != nil {
			if ref.AuthToken == nil {
				return nil, fmt.Errorf("census is locked")
			}
			if !bytes.Equal(authToken[:], ref.AuthToken[:]) {
				return nil, fmt.Errorf("wrong authentication token")
			}
		}
		return &ref, nil
	}

	// if not in memory, load census from DB
	ref, err := a.getCensusRefFromDB(censusID)
	if err != nil {
		return nil, err
	}
	// check authentication
	if authToken != nil {
		// if no token stored in the reference but the called provided a token, we don't allow
		if ref.AuthToken == nil {
			return nil, fmt.Errorf("census is locked")
		}
		if !bytes.Equal(authToken[:], ref.AuthToken[:]) {
			return nil, fmt.Errorf("wrong authentication token")
		}
	}
	ref.tree, err = censustree.New(censustree.Options{Name: censusName(censusID), ParentDB: a.db,
		MaxLevels: 256, CensusType: models.Census_Type(ref.CensusType)})
	if err != nil {
		return nil, err
	}
	if ref.IsPublic {
		ref.tree.Publish()
	}
	a.censusMap.Store(string(censusID), *ref)
	log.Debugf("loaded tree %x", censusID)
	return ref, nil
}

func (a *API) addCensusRefToDB(censusID []byte, authToken *uuid.UUID,
	t models.Census_Type, indexed, public bool) (*censusRef, error) {
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	wtx := a.db.WriteTx()
	defer wtx.Discard()
	refData := bytes.Buffer{}
	enc := gob.NewEncoder(&refData)
	ref := &censusRef{
		AuthToken:  authToken,
		CensusType: int32(t),
		Indexed:    indexed,
		IsPublic:   public,
	}
	if err := enc.Encode(ref); err != nil {
		return nil, err
	}
	if err := wtx.Set(append([]byte(censusDBreferencePrefix), censusID...),
		refData.Bytes()); err != nil {
		return nil, err
	}
	return ref, wtx.Commit()
}

func (a *API) delCensusRefFromDB(censusID []byte) error {
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	wtx := a.db.WriteTx()
	defer wtx.Discard()
	if err := wtx.Delete(append([]byte(censusDBreferencePrefix), censusID...)); err != nil {
		return err
	}
	a.censusMap.Delete(string(censusID))
	return wtx.Commit()
}

func (a *API) getCensusRefFromDB(censusID []byte) (*censusRef, error) {
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	b, err := a.db.ReadTx().Get(
		append(
			[]byte(censusDBreferencePrefix),
			censusID...,
		))
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, fmt.Errorf("census id not found in local storage")
		}
		return nil, err
	}
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	ref := censusRef{}
	return &ref, dec.Decode(&ref)
}
