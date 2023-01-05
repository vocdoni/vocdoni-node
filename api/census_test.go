package api

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"path"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/proto/build/go/models"
)

func TestCensus(t *testing.T) {
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(t, err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := data.MockStorage(t)
	api.Attach(nil, nil, nil, data.Storage(&storage), censusDB)
	qt.Assert(t, api.EnableHandlers(CensusHandler), qt.IsNil)

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, addr, &token1)

	// create a new census
	resp, code := c.Request("POST", nil, "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// check weight and size (must be zero)
	resp, code = c.Request("GET", nil, id1, "weight")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "0")

	resp, code = c.Request("GET", nil, id1, "size")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Size, qt.Equals, uint64(0))

	// add a bunch of keys and values (weights)
	rnd := testutil.NewRandom(1)
	weight := 0
	index := uint64(0)
	cparts := CensusParticipants{}
	for i := 1; i < 11; i++ {
		cparts.Participants = append(cparts.Participants, CensusParticipant{
			Key:    rnd.RandomBytes(32),
			Weight: (*types.BigInt)(big.NewInt(int64(i))),
		})
		weight += i
		index++
	}
	_, code = c.Request("POST", &cparts, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	// check again weight and size
	resp, code = c.Request("GET", nil, id1, "weight")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, fmt.Sprintf("%d", weight))

	resp, code = c.Request("GET", nil, id1, "size")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Size, qt.Equals, index)

	// add one more key and check proof
	key := rnd.RandomBytes(32)
	keyWeight := (*types.BigInt)(big.NewInt(100))
	_, code = c.Request("POST", &CensusParticipants{
		Participants: []CensusParticipant{{Key: key, Weight: keyWeight}},
	}, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request("POST", nil, id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	id2 := censusData.CensusID.String()

	resp, code = c.Request("GET", nil, id2, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, keyWeight.String())

	proof := &Census{
		Key:   key,
		Proof: censusData.Proof,
		Value: censusData.Value,
	}
	resp, code = c.Request("POST", proof, id2, "verify")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Valid, qt.IsTrue)

	// try an invalid proof
	proof.Key = rnd.RandomBytes(32)
	qt.Assert(t, err, qt.IsNil)
	_, code = c.Request("POST", proof, id2, "verify")
	qt.Assert(t, code, qt.Equals, 400)

	// dump the tree
	censusDump := &censusdb.CensusDump{}
	resp, code = c.Request("GET", nil, id1, "export")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusDump), qt.IsNil)

	// create a new one and try to import the previous dump
	resp2, code := c.Request("POST", nil, "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp2, censusData), qt.IsNil)
	id3 := censusData.CensusID.String()

	resp, code = c.Request("POST", censusDump, id3, "import")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// delete the first census
	_, code = c.Request("DELETE", nil, id1)
	qt.Assert(t, code, qt.Equals, 200)

	// check the second census is still available
	_, code = c.Request("GET", nil, id2, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 200)

	// check the first census is not available
	_, code = c.Request("GET", nil, id1, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 400)
}

func TestCensusProof(t *testing.T) {
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(t, err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	// Create local key value database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	api.Attach(nil, nil, nil, nil, censusDB)
	qt.Assert(t, api.EnableHandlers(CensusHandler), qt.IsNil)

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, addr, &token1)

	// create a new census
	resp, code := c.Request("POST", nil, CensusTypeWeighted)
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// add a bunch of keys and values (weights)
	rnd := testutil.NewRandom(1)
	cparts := CensusParticipants{}
	for i := 1; i < 11; i++ {
		cparts.Participants = append(cparts.Participants, CensusParticipant{
			Key:    rnd.RandomBytes(32),
			Weight: (*types.BigInt)(big.NewInt(int64(i))),
		})
	}
	_, code = c.Request("POST", &cparts, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	// add the last participant and keep the key for verifying the proof
	key := rnd.RandomBytes(32)
	_, code = c.Request("POST", &CensusParticipants{Participants: []CensusParticipant{{
		Key:    key,
		Weight: (*types.BigInt)(big.NewInt(1)),
	}}}, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request("POST", nil, id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID.String()

	resp, code = c.Request("GET", nil, root, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")

	// verify the proof
	electionID := rnd.RandomBytes(32)
	valid, weight, err := transaction.VerifyProof(
		&models.Process{
			ProcessId:    electionID,
			CensusRoot:   censusData.CensusID,
			CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		},
		&models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: censusData.Proof,
					Value:    censusData.Value,
				},
			},
		},
		models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		censusData.CensusID,
		electionID,
		key,
		common.Address{},
	)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, valid, qt.IsTrue)
	qt.Assert(t, weight.Uint64(), qt.Equals, uint64(1))
}

func TestCensusZk(t *testing.T) {
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(t, err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := data.MockStorage(t)
	api.Attach(nil, nil, nil, data.Storage(&storage), censusDB)
	qt.Assert(t, api.EnableHandlers(CensusHandler), qt.IsNil)

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, addr, &token1)

	// create a new zk census
	resp, code := c.Request("POST", nil, CensusTypeZKWeighted)
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// add a bunch of keys and values (weights)
	keys := [][]byte{}
	for i := 1; i < 11; i++ {
		k := babyjub.NewRandPrivKey()
		publicKeyHash, err := poseidon.Hash([]*big.Int{
			k.Public().X,
			k.Public().Y,
		})
		qt.Assert(t, err, qt.IsNil)
		keys = append(keys, arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), publicKeyHash))
	}

	weight := 0
	index := uint64(0)
	cparts := CensusParticipants{}
	for i := 1; i < 11; i++ {
		cparts.Participants = append(cparts.Participants, CensusParticipant{
			Key:    keys[i-1],
			Weight: (*types.BigInt)(big.NewInt(int64(i))),
		})
		weight += i
		index++
	}
	_, code = c.Request("POST", &cparts, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	// check weight and size
	resp, code = c.Request("GET", nil, id1, "weight")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, fmt.Sprintf("%d", weight))

	resp, code = c.Request("GET", nil, id1, "size")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Size, qt.Equals, index)

	// publish the census
	resp, code = c.Request("POST", nil, id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID.String()

	// generate a proof
	resp, code = c.Request("GET", nil, root, "proof", fmt.Sprintf("%x", keys[0]))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")
}
