package api

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"path"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/util"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/proto/build/go/models"
)

func TestCensus(t *testing.T) {
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(t, err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	qt.Assert(t, err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := ipfs.MockIPFS(t)
	app := vochain.TestBaseApplication(t)
	api.Attach(app, nil, nil, storage, censusDB)
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
			Key:    rnd.RandomBytes(20),
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
	key := rnd.RandomBytes(20)
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
		Key:         key,
		CensusProof: censusData.CensusProof,
		Value:       censusData.Value,
	}
	resp, code = c.Request("POST", proof, id2, "verify")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Valid, qt.IsTrue)

	// try an invalid proof
	proof.Key = rnd.RandomBytes(20)
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
	qt.Assert(t, code, qt.Equals, 404)
}

func TestCensusProof(t *testing.T) {
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(t, err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	qt.Assert(t, err, qt.IsNil)
	// Create local key value database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	app := vochain.TestBaseApplication(t)
	api.Attach(app, nil, nil, nil, censusDB)
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
	for i, acc := range ethereum.NewSignKeysBatch(11) {
		cparts.Participants = append(cparts.Participants, CensusParticipant{
			Key:    acc.Address().Bytes(),
			Weight: (*types.BigInt)(big.NewInt(int64(i))),
		})
	}
	_, code = c.Request("POST", &cparts, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	// add the last participant and keep the key for verifying the proof
	key := ethereum.NewSignKeys()
	qt.Assert(t, key.Generate(), qt.IsNil)
	_, code = c.Request("POST", &CensusParticipants{Participants: []CensusParticipant{{
		Key:    key.Address().Bytes(),
		Weight: (*types.BigInt)(big.NewInt(1)),
	}}}, id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request("POST", nil, id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID.String()

	resp, code = c.Request("GET", nil, root, "proof", key.Address().String())
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
			EnvelopeType: &models.EnvelopeType{},
		},
		&models.VoteEnvelope{
			ProcessId: electionID,
			Proof: &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:            models.ProofArbo_BLAKE2B,
						Siblings:        censusData.CensusProof,
						AvailableWeight: censusData.Value,
						VoteWeight:      censusData.Value,
					},
				},
			},
		},
		state.NewVoterID(state.VoterIDTypeECDSA, key.PublicKey()),
	)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, valid, qt.IsTrue)
	qt.Assert(t, weight.Uint64(), qt.Equals, uint64(1))
}

func TestCensusZk(t *testing.T) {
	c := qt.New(t)

	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	c.Assert(err, qt.IsNil)

	testApi, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	c.Assert(err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	c.Assert(err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := ipfs.MockIPFS(t)
	app := vochain.TestBaseApplication(t)
	testApi.Attach(app, nil, nil, storage, censusDB)
	c.Assert(testApi.EnableHandlers(CensusHandler), qt.IsNil)

	token1 := uuid.New()
	httpClient := testutil.NewTestHTTPclient(t, addr, &token1)

	// create a new zk census
	resp, code := httpClient.Request("POST", nil, CensusTypeZKWeighted)
	c.Assert(code, qt.Equals, 200)
	censusData := &Census{}
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// add a bunch of keys and values (weights)
	var accounts []*ethereum.SignKeys
	var keys [][]byte
	for i := 1; i < 11; i++ {
		acc := ethereum.NewSignKeys()
		c.Assert(acc.Generate(), qt.IsNil)
		accounts = append(accounts, acc)
		keys = append(keys, acc.Address().Bytes())
		// generate sik and create the vochain account for this voter with it
		sik, err := acc.AccountSIK(nil)
		c.Assert(err, qt.IsNil)
		c.Assert(testApi.vocapp.State.SetAddressSIK(acc.Address(), sik), qt.IsNil)
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
	_, code = httpClient.Request("POST", &cparts, id1, "participants")
	c.Assert(code, qt.Equals, 200)

	// check weight and size
	resp, code = httpClient.Request("GET", nil, id1, "weight")
	c.Assert(code, qt.Equals, 200)
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	c.Assert(censusData.Weight.String(), qt.Equals, fmt.Sprintf("%d", weight))

	resp, code = httpClient.Request("GET", nil, id1, "size")
	c.Assert(code, qt.Equals, 200)
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	c.Assert(censusData.Size, qt.Equals, index)

	// publish the census
	resp, code = httpClient.Request("POST", nil, id1, "publish")
	c.Assert(code, qt.Equals, 200)
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	c.Assert(censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID.String()

	// generate a proof
	resp, code = httpClient.Request("GET", nil, root, "proof", accounts[0].AddressString())
	c.Assert(code, qt.Equals, 200)
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	c.Assert(censusData.Weight.String(), qt.Equals, "1")

	// verify the proof
	electionID := util.RandomBytes(32)
	valid, newWeight, err := transaction.VerifyProof(
		&models.Process{
			ProcessId:    electionID,
			CensusRoot:   censusData.CensusID,
			CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
			EnvelopeType: &models.EnvelopeType{},
		},
		&models.VoteEnvelope{
			ProcessId: electionID,
			Proof: &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:            models.ProofArbo_POSEIDON,
						Siblings:        censusData.CensusProof,
						AvailableWeight: censusData.Value,
						VoteWeight:      censusData.Value,
					},
				},
			},
		},
		state.NewVoterID(state.VoterIDTypeECDSA, accounts[0].PublicKey()),
	)

	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, valid, qt.IsTrue)
	qt.Assert(t, newWeight.Uint64(), qt.Equals, uint64(1))
}
