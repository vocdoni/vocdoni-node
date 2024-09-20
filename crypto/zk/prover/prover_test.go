package prover

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"

	qt "github.com/frankban/quicktest"
)

var (
	wasm        = getExampleFile("./test_files/circuit.wasm")
	wasm2       = getExampleFile("./test_files/circuit2.wasm")
	inputs      = getExampleFile("./test_files/inputs.json")
	inputs2     = getExampleFile("./test_files/inputs2.json")
	zkey        = getExampleFile("./test_files/proving_key.zkey")
	zkey2       = getExampleFile("./test_files/proving_key2.zkey")
	pubSignals  = getExampleFile("./test_files/public_signals.json")
	pubSignals2 = getExampleFile("./test_files/public_signals2.json")
	vkey        = getExampleFile("./test_files/verification_key.json")
	vkey2       = getExampleFile("./test_files/verification_key2.json")
)

// getExampleFile is a helper function to read local files that are used into
// the tests. It logs an error if something was wrong.
func getExampleFile(path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("no circuit found on '%s'", path)
	}
	return content
}

func TestParseProof(t *testing.T) {
	proof, _ := Prove(zkey, wasm, inputs)
	validProofData, validPubSignals, _ := proof.Bytes()

	result, err := ParseProof(validProofData, validPubSignals)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result.Data, qt.ContentEquals, proof.Data)
	qt.Assert(t, result.PubSignals, qt.ContentEquals, proof.PubSignals)

	_, err = ParseProof([]byte{}, validPubSignals)
	qt.Assert(t, err, qt.IsNotNil)
	_, err = ParseProof(validProofData, []byte{})
	qt.Assert(t, err, qt.IsNotNil)
}

func TestBytes(t *testing.T) {
	expected, _ := Prove(zkey, wasm, inputs)

	validProofData, validPubSignals, err := expected.Bytes()
	qt.Assert(t, err, qt.IsNil)
	result, err := ParseProof(validProofData, validPubSignals)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.DeepEquals, expected)

	expectedProofData, err := json.Marshal(expected.Data)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, validProofData, qt.DeepEquals, expectedProofData)
	expectedPubSignals, err := json.Marshal(expected.PubSignals)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, validPubSignals, qt.DeepEquals, expectedPubSignals)
}

func Test_calcWitness(t *testing.T) {
	// Empty and first set of valid parameters
	emptyWasm, emptyInputs := []byte{}, []byte{}

	_, err := calcWitness(emptyWasm, inputs)
	qt.Assert(t, err, qt.IsNotNil)

	_, err = calcWitness(wasm, emptyInputs)
	qt.Assert(t, err, qt.IsNotNil)

	wtns, err := calcWitness(wasm, inputs)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, wtns, qt.Not(qt.HasLen), 0)

	// Wrong and second set of valid parameters
	_, err = calcWitness(wasm, inputs2)
	qt.Assert(t, err, qt.IsNotNil)

	_, err = calcWitness(wasm2, inputs)
	qt.Assert(t, err, qt.IsNotNil)

	wtns, err = calcWitness(wasm2, inputs2)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, wtns, qt.Not(qt.HasLen), 0)
}

func TestProve(t *testing.T) {
	// Empty and valid parameters
	_, err := Prove([]byte{}, wasm, inputs)
	qt.Assert(t, err, qt.IsNotNil)

	validPubSignals, validPubSignals2 := []string{}, []string{}
	_ = json.Unmarshal(pubSignals, &validPubSignals)
	_ = json.Unmarshal(pubSignals2, &validPubSignals2)

	proof, err := Prove(zkey, wasm, inputs)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proof.PubSignals, qt.ContentEquals, validPubSignals)

	// Second set of valid parameters
	proof, err = Prove(zkey2, wasm2, inputs2)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proof.PubSignals, qt.ContentEquals, validPubSignals2)
}

func TestVerify(t *testing.T) {
	// Check a valid case
	proof, _ := Prove(zkey, wasm, inputs)
	err := proof.Verify(vkey)
	qt.Assert(t, err, qt.IsNil)

	// Check an invalid case with empty parameters
	err = proof.Verify([]byte{})
	qt.Assert(t, err, qt.IsNotNil)

	wrongProof := &Proof{Data: ProofData{}, PubSignals: proof.PubSignals}
	err = wrongProof.Verify(vkey)
	qt.Assert(t, err, qt.IsNotNil)

	wrongProof = &Proof{Data: proof.Data, PubSignals: []string{}}
	err = wrongProof.Verify(vkey)
	qt.Assert(t, err, qt.IsNotNil)

	// Check a proof generated with a different zkey
	wrongProof, _ = Prove(zkey2, wasm, inputs)
	err = wrongProof.Verify(vkey)
	qt.Assert(t, err, qt.IsNotNil)

	err = wrongProof.Verify(vkey2)
	qt.Assert(t, err, qt.IsNotNil)
}

func TestGnark(t *testing.T) {
	// Generate proof
	proof, _ := Prove(zkey, wasm, inputs)

	gnarkproof := ProofToGnarkProof(proof)

	sp1proof := GnarkProofToSP1Proof(gnarkproof, WitnessInput{"1234", "5678"})

	t.Logf("%+v", sp1proof)
}

func ProofToGnarkProof(proof *Proof) *groth16.Proof {
}

// WitnessInput is copypasta from sp1
type WitnessInput struct {
	VkeyHash             string `json:"vkey_hash"`
	CommitedValuesDigest string `json:"commited_values_digest"`
}

// SP1Proof is copypasta from sp1
type SP1Proof struct {
	PublicInputs [2]string `json:"public_inputs"`
	EncodedProof string    `json:"encoded_proof"`
	RawProof     string    `json:"raw_proof"`
}

func GnarkProofToSP1Proof(proof *groth16.Proof, witnessInput WitnessInput) SP1Proof {
	var publicInputs [2]string
	publicInputs[0] = witnessInput.VkeyHash
	publicInputs[1] = witnessInput.CommitedValuesDigest

	// Cast groth16 proof into groth16_bn254 proof so we can call MarshalSolidity.
	p := (*proof).(*groth16_bn254.Proof)

	encodedProof := p.MarshalSolidity()

	return SP1Proof{
		PublicInputs: publicInputs,
		EncodedProof: hex.EncodeToString(encodedProof),
		// RawProof:     hex.EncodeToString(proofBytes), // this field is uninteresting AFAIU
	}
}
