package urlapi

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
	case "zkindexed":
		return models.Census_ARBO_POSEIDON, true
	case "weighted":
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

func (u *URLAPI) createNewCensus(censusID []byte, censusType models.Census_Type,
	indexed, public bool, authToken *uuid.UUID) (*censusRef, error) {
	tree, err := censustree.New(censustree.Options{Name: censusName(censusID), ParentDB: u.db,
		MaxLevels: 256, CensusType: censusType, IndexAsKeysCensus: indexed})
	if err != nil {
		return nil, err
	}
	ref, err := u.addCensusRefToDB(censusID, authToken, censusType, indexed, public)
	if err != nil {
		return nil, err
	}
	ref.tree = tree
	if public {
		ref.tree.Publish()
	}
	// add the tree in memory so we can quickly access to it afterwards
	u.censusMap.Store(string(censusID), *ref)
	return ref, nil
}

func (u *URLAPI) censusRefExist(censusID []byte) bool {
	_, ok := u.censusMap.Load(string(censusID))
	return ok
}

// loadCensus returns an already loaded census from memory or from the persistent kv database.
// Authentication is checked if authToken is not nil.
func (u *URLAPI) loadCensus(censusID []byte, authToken *uuid.UUID) (*censusRef, error) {
	// if the tree is in memory, just return it
	val, ok := u.censusMap.Load(string(censusID))
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
	ref, err := u.getCensusRefFromDB(censusID)
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
	ref.tree, err = censustree.New(censustree.Options{Name: censusName(censusID), ParentDB: u.db,
		MaxLevels: 256, CensusType: models.Census_Type(ref.CensusType)})
	if err != nil {
		return nil, err
	}
	if ref.IsPublic {
		ref.tree.Publish()
	}
	u.censusMap.Store(string(censusID), *ref)
	log.Debugf("loaded tree %x", censusID)
	return ref, nil
}

func (u *URLAPI) addCensusRefToDB(censusID []byte, authToken *uuid.UUID,
	t models.Census_Type, indexed, public bool) (*censusRef, error) {
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	wtx := u.db.WriteTx()
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

func (u *URLAPI) delCensusRefFromDB(censusID []byte) error {
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	wtx := u.db.WriteTx()
	defer wtx.Discard()
	if err := wtx.Delete(append([]byte(censusDBreferencePrefix), censusID...)); err != nil {
		return err
	}
	u.censusMap.Delete(string(censusID))
	return wtx.Commit()
}

func (u *URLAPI) getCensusRefFromDB(censusID []byte) (*censusRef, error) {
	censusDBlock.Lock()
	defer censusDBlock.Unlock()
	b, err := u.db.ReadTx().Get(
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
