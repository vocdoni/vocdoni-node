package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/url"
	"path"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

func init() { rand.Seed(time.Now().UnixNano()) }

var zkCircuitTest = circuit.CircuitsConfigurations["dev"]

// go test -v -run=- -bench=BenchmarkCensus -benchmem -count=100 .
func BenchmarkCensus(b *testing.B) {
	b.ReportAllocs()
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "censuses"))
	qt.Assert(b, err, qt.IsNil)

	api, err := NewAPI(&router, "/", b.TempDir())
	qt.Assert(b, err, qt.IsNil)

	// Create local key value database
	db, err := metadb.New(db.TypePebble, b.TempDir())
	qt.Assert(b, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	storage := data.MockIPFS(b)
	api.Attach(nil, nil, nil, storage, censusDB)
	qt.Assert(b, api.EnableHandlers(CensusHandler), qt.IsNil)

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(b, addr, &token1)

	// create a new zk census
	resp, code := c.Request("POST", nil, CensusTypeZKWeighted)
	qt.Assert(b, code, qt.Equals, 200)
	censusData := &Census{}
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	censusID := censusData.CensusID.String()
	electionID := util.RandomBytes(32)

	b.ResetTimer()

	b.Run("census", func(b *testing.B) {
		zkCensusBenchmark(b, c, censusID, electionID)
	})
}

func zkCensusBenchmark(b *testing.B, cl *testutil.TestHTTPclient, censusID string, electionID []byte) {
	k := babyjub.NewRandPrivKey()
	publicKeyHash, err := poseidon.Hash([]*big.Int{
		k.Public().X,
		k.Public().Y,
	})
	qt.Assert(b, err, qt.IsNil)
	pubkey := arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), publicKeyHash)

	cparts := CensusParticipants{}
	cparts.Participants = append(cparts.Participants, CensusParticipant{
		Key:    pubkey,
		Weight: (*types.BigInt)(big.NewInt(1)),
	})

	_, code := cl.Request("POST", &cparts, censusID, "participants")
	qt.Assert(b, code, qt.Equals, 200)

	var resp []byte
	censusData := &Census{}
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
	resp, code = cl.Request("GET", nil, censusID, "proof", fmt.Sprintf("%x", pubkey))
	qt.Assert(b, code, qt.Equals, 200)
	qt.Assert(b, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(b, censusData.Weight.String(), qt.Equals, "1")

	censusData.Root = root
	genProofZk(b, electionID, &k, pubkey, censusData)
}

func genProofZk(b *testing.B, electionID []byte, privKey *babyjub.PrivateKey, pubKey []byte, censusData *Census) {
	strPrivateKey := babyjub.SkToBigInt(privKey).String()
	// Get merkle proof associated to the voter key provided, that will contains
	// the leaf siblings and value (weight)

	log.Infow("zk census data received, starting to generate the proof inputs...", map[string]interface{}{
		"censusRoot": censusData.Root, "electionId": fmt.Sprintf("%x", electionID)})

	// Encode census root
	strCensusRoot := arbo.BytesToBigInt(censusData.Root).String()
	// Get vote weight
	weight := new(big.Int).SetInt64(1)
	if censusData.Weight != nil {
		weight = censusData.Weight.ToInt()
	}
	// Get nullifier and encoded processId
	nullifier, strProcessId, err := getNullifierZk(privKey, electionID)
	qt.Assert(b, err, qt.IsNil)

	strNullifier := new(big.Int).SetBytes(nullifier).String()
	// Calculate and encode vote hash -> sha256(voteWeight)
	voteHash := sha256.Sum256(censusData.Value)
	strVoteHash := []string{
		new(big.Int).SetBytes(arbo.SwapEndianness(voteHash[:16])).String(),
		new(big.Int).SetBytes(arbo.SwapEndianness(voteHash[16:])).String(),
	}
	// Unpack and encode siblings
	unpackedSiblings, err := arbo.UnpackSiblings(arbo.HashFunctionPoseidon, censusData.Proof)
	qt.Assert(b, err, qt.IsNil)

	// Create a list of siblings with the same number of items that levels
	// allowed by the circuit (from its config) plus one. Fill with zeros if its
	// needed.
	strSiblings := make([]string, zkCircuitTest.Levels+1)
	for i := 0; i < len(strSiblings); i++ {
		newSibling := "0"
		if i < len(unpackedSiblings) {
			newSibling = arbo.BytesToBigInt(unpackedSiblings[i]).String()
		}
		strSiblings[i] = newSibling
	}
	// Get artifacts of the current circuit
	currentCircuit, err := circuit.LoadZkCircuit(context.Background(), zkCircuitTest)
	qt.Assert(b, err, qt.IsNil)

	// Create the inputs and encode them into a JSON
	rawInputs := map[string]interface{}{
		"censusRoot":     strCensusRoot,
		"censusSiblings": strSiblings,
		"weight":         weight.String(),
		"privateKey":     strPrivateKey,
		"voteHash":       strVoteHash,
		"processId":      strProcessId,
		"nullifier":      strNullifier,
	}
	inputs, err := json.Marshal(rawInputs)
	qt.Assert(b, err, qt.IsNil)

	log.Infow("proof inputs generated", map[string]interface{}{
		"censusRoot": censusData.Root.String(), "nullifier": strNullifier})
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

func getNullifierZk(privKey *babyjub.PrivateKey, electionId []byte) (types.HexBytes, []string, error) {
	// Encode the electionId -> sha256(electionId)
	hashedProcessId := sha256.Sum256(electionId)
	intProcessId := []*big.Int{
		new(big.Int).SetBytes(arbo.SwapEndianness(hashedProcessId[:16])),
		new(big.Int).SetBytes(arbo.SwapEndianness(hashedProcessId[16:])),
	}
	strProcessId := []string{intProcessId[0].String(), intProcessId[1].String()}

	// Calculate nullifier hash: poseidon(babyjubjub(privKey) + sha256(processId))
	nullifier, err := poseidon.Hash([]*big.Int{
		babyjub.SkToBigInt(privKey),
		intProcessId[0],
		intProcessId[1],
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error generating nullifier: %w", err)
	}
	return nullifier.Bytes(), strProcessId, nil
}
