package prover

import (
	"log"
	"os"
	"testing"

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

func Test_calcWitness(t *testing.T) {
	// Empty and first set of valid parameters
	var emptyWasm, emptyInputs = []byte{}, []byte{}

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
	_, _, err := Prove([]byte{}, wasm, inputs)
	qt.Assert(t, err, qt.IsNotNil)

	_, resPubSignals, err := Prove(zkey, wasm, inputs)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, resPubSignals, qt.ContentEquals, pubSignals)

	// Second set of valid parameters
	_, resPubSignals, err = Prove(zkey2, wasm2, inputs2)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, resPubSignals, qt.ContentEquals, pubSignals2)
}

func TestVerify(t *testing.T) {
	// Check a valid case
	validProofData, validPublicSignals, _ := Prove(zkey, wasm, inputs)
	err := Verify(vkey, validProofData, validPublicSignals)
	qt.Assert(t, err, qt.IsNil)

	// Check an invalid case with empty parameters
	err = Verify([]byte{}, validProofData, validPublicSignals)
	qt.Assert(t, err, qt.IsNotNil)

	err = Verify(vkey, []byte{}, validPublicSignals)
	qt.Assert(t, err, qt.IsNotNil)

	err = Verify(vkey, validProofData, []byte{})
	qt.Assert(t, err, qt.IsNotNil)

	// Check a proof generated with a different zkey
	invalidProofData, invalidPublicSignals, _ := Prove(zkey2, wasm, inputs)
	err = Verify(vkey, invalidProofData, invalidPublicSignals)
	qt.Assert(t, err, qt.IsNotNil)

	err = Verify(vkey2, invalidProofData, invalidPublicSignals)
	qt.Assert(t, err, qt.IsNotNil)
}
