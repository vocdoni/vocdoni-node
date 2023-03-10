package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"path"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
)

var zkCircuitTest = circuit.CircuitsConfigurations["dev"]

// go test -v -run=- -bench=BenchmarkZkCensus -benchmem -count=100 .
func BenchmarkZkCensus(b *testing.B) {
	b.ReportAllocs()
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(b, err, qt.IsNil)

	vocApi, err := api.NewAPI(&router, "/", b.TempDir())
	qt.Assert(b, err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, b.TempDir())
	qt.Assert(b, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := data.MockIPFS(b)

	vocapp := vochain.TestBaseApplication(b)
	vocApi.Attach(vocapp, nil, nil, storage, censusDB)
	qt.Assert(b, vocApi.EnableHandlers(api.CensusHandler), qt.IsNil)

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(b, addr, &token1)

	// create a new zk census
	resp, code := c.Request("POST", nil, api.CensusTypeZKWeighted)
	qt.Assert(b, code, qt.Equals, 200)
	censusData := &api.Census{}
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	censusID := censusData.CensusID.String()
	electionID := util.RandomBytes(32)

	b.ResetTimer()

	b.Run("census", func(b *testing.B) {
		zkCensusBenchmark(b, c, censusID, electionID)
	})
}

func zkCensusBenchmark(b *testing.B, cl *testutil.TestHTTPclient, censusID string, electionID []byte) {
	zkAddr, err := zk.NewRandAddress()
	qt.Assert(b, err, qt.IsNil)

	cparts := api.CensusParticipants{}
	cparts.Participants = append(cparts.Participants, api.CensusParticipant{
		Key:    zkAddr.Bytes(),
		Weight: (*types.BigInt)(big.NewInt(1)),
	})

	_, code := cl.Request("POST", &cparts, censusID, "participants")
	qt.Assert(b, code, qt.Equals, 200)

	var resp []byte
	censusData := &api.Census{}
	resp, code = cl.Request("GET", nil, censusID, "root")
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(b, censusData.Root, qt.IsNotNil)
	root := censusData.Root

	resp, code = cl.Request("GET", nil, censusID, "size")
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	b.Logf("root: %x | size: %d", root, censusData.Size)

	// generate a proof
	resp, code = cl.Request("GET", nil, censusID, "proof", zkAddr.String())
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(b, censusData.Weight.String(), qt.Equals, "1")

	censusData.Root = root
	genProofZk(b, electionID, zkAddr, censusData)
}

func genProofZk(b *testing.B, electionID []byte, zkAddr *zk.ZkAddress, censusData *api.Census) {
	// Get merkle proof associated to the voter key provided, that will contains
	// the leaf siblings and value (weight)

	log.Infow("zk census data received, starting to generate the proof inputs...",
		"censusRoot", censusData.Root, "electionId", fmt.Sprintf("%x", electionID))

	// Get vote weight
	weight := new(big.Int).SetInt64(1)
	if censusData.Weight != nil {
		weight = censusData.Weight.MathBigInt()
	}
	// Generate circuit inputs
	rawInputs, err := circuit.GenerateCircuitInput(zkAddr, censusData.Root, electionID, weight, weight, censusData.Siblings)
	qt.Assert(b, err, qt.IsNil)
	// Encode the inputs into a JSON
	inputs, err := json.Marshal(rawInputs)
	qt.Assert(b, err, qt.IsNil)
	// parse nullifier from generated inputs
	nullifier, ok := new(big.Int).SetString(rawInputs.Nullifier, 10)
	qt.Assert(b, ok, qt.IsTrue)

	log.Infow("proof inputs generated", "censusRoot", censusData.Root.String(),
		"nullifier", nullifier.String())

	// Get artifacts of the current circuit
	currentCircuit, err := circuit.LoadZkCircuit(context.Background(), zkCircuitTest)
	qt.Assert(b, err, qt.IsNil)
	// Calculate the proof for the current apiclient circuit config and the
	// inputs encoded.
	proof, err := prover.Prove(currentCircuit.ProvingKey, currentCircuit.Wasm, inputs)
	qt.Assert(b, err, qt.IsNil)
	// Encode the results as bytes and return the proof
	encProof, encPubSignals, err := proof.Bytes()
	qt.Assert(b, err, qt.IsNil)
	qt.Assert(b, encProof, qt.Not(qt.IsNil))
	qt.Assert(b, encPubSignals, qt.Not(qt.IsNil))
}
