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
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/data/ipfs"
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

	vocApi, err := api.NewAPI(&router, "/", b.TempDir(), db.TypePebble)
	qt.Assert(b, err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, b.TempDir())
	qt.Assert(b, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := ipfs.MockIPFS(b)
	vocapp := vochain.TestBaseApplication(b)
	vocApi.Attach(vocapp, nil, nil, storage, censusDB)
	qt.Assert(b, vocApi.EnableHandlers(api.CensusHandler, api.SIKHandler), qt.IsNil)

	censusUrl, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(b, err, qt.IsNil)
	token := uuid.New()
	censusClient := testutil.NewTestHTTPclient(b, censusUrl, &token)

	sikUrl, err := url.Parse("http://" + path.Join(router.Address().String(), "sik"))
	qt.Assert(b, err, qt.IsNil)
	sikClient := testutil.NewTestHTTPclient(b, sikUrl, &token)

	// create a new zk census
	resp, code := censusClient.Request("POST", nil, api.CensusTypeZKWeighted)
	qt.Assert(b, code, qt.Equals, 200)
	censusData := &api.Census{}
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	censusID := censusData.CensusID.String()
	electionID := util.RandomBytes(32)

	b.ResetTimer()

	b.Run("census", func(b *testing.B) {
		zkCensusBenchmark(b, censusClient, sikClient, vocapp, censusID, electionID)
	})
}

func zkCensusBenchmark(b *testing.B, cc, sc *testutil.TestHTTPclient, vapp *vochain.BaseApplication, censusID string, electionID []byte) {
	cparts := api.CensusParticipants{}
	admin := ethereum.NewSignKeys()
	for i := 0; i < 10; i++ {
		acc := admin
		if i != 0 {
			acc = ethereum.NewSignKeys()
		}
		qt.Assert(b, acc.Generate(), qt.IsNil)
		sik, err := acc.AccountSIK(nil)
		qt.Assert(b, err, qt.IsNil)
		qt.Assert(b, vapp.State.SetAddressSIK(acc.Address(), sik), qt.IsNil)

		cparts.Participants = append(cparts.Participants, api.CensusParticipant{
			Key:    acc.Address().Bytes(),
			Weight: (*types.BigInt)(big.NewInt(1)),
		})
	}

	_, code := cc.Request("POST", &cparts, censusID, "participants")
	qt.Assert(b, code, qt.Equals, 200)

	var resp []byte
	censusData := &api.Census{}
	resp, code = cc.Request("GET", nil, censusID, "root")
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(b, censusData.CensusRoot, qt.IsNotNil)
	root := censusData.CensusRoot

	resp, code = cc.Request("GET", nil, censusID, "size")
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	b.Logf("root: %x | size: %d", root, censusData.Size)

	// generate a proof
	resp, code = cc.Request("GET", nil, censusID, "proof", admin.AddressString())
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(b, censusData.Weight.String(), qt.Equals, "1")
	censusData.CensusRoot = root

	// generate a sik proof
	sikData := &api.Census{}
	resp, code = sc.Request("GET", nil, "proof", admin.AddressString())
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, sikData), qt.IsNil)

	genProofZk(b, electionID, admin, censusData, sikData)
}

func genProofZk(b *testing.B, electionID []byte, acc *ethereum.SignKeys, censusData, sikData *api.Census) {
	// Get merkle proof associated to the voter key provided, that will contains
	// the leaf siblings and value (weight)
	log.Infow("zk census data received, starting to generate the proof inputs...",
		"censusRoot", censusData.CensusRoot, "electionId", fmt.Sprintf("%x", electionID))

	// Get vote weight
	weight := new(big.Int).SetInt64(1)
	if censusData.Weight != nil {
		weight = censusData.Weight.MathBigInt()
	}
	testingVotePackage := util.RandomBytes(16)
	// Generate circuit inputs
	rawInputs, err := circuit.GenerateCircuitInput(circuit.CircuitInputsParameters{
		Account:         acc,
		ElectionId:      electionID,
		CensusRoot:      censusData.CensusRoot,
		SIKRoot:         sikData.CensusRoot,
		VotePackage:     testingVotePackage,
		CensusSiblings:  censusData.CensusSiblings,
		SIKSiblings:     sikData.CensusSiblings,
		VoteWeight:      weight,
		AvailableWeight: censusData.Weight.MathBigInt(),
	})
	qt.Assert(b, err, qt.IsNil)
	// Encode the inputs into a JSON
	inputs, err := json.Marshal(rawInputs)
	qt.Assert(b, err, qt.IsNil)
	// parse nullifier from generated inputs
	nullifier, ok := new(big.Int).SetString(rawInputs.Nullifier, 10)
	qt.Assert(b, ok, qt.IsTrue)

	log.Infow("proof inputs generated", "censusRoot", censusData.CensusRoot.String(),
		"nullifier", nullifier.String())

	// Get artifacts of the current circuit
	currentCircuit, err := circuit.LoadConfig(context.Background(), zkCircuitTest)
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
