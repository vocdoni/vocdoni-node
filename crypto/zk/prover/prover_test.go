package prover

import (
	"os"
	"reflect"
	"testing"
)

// getExampleFile is a helper function to read local file that is used like test
// example. It rais an error if something was wrong that makes that the current
// test fails.
func getExampleFile(t *testing.T, path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no circuit found on '%s'", path)
	}

	return content
}

func Test_calcWitness(t *testing.T) {
	var (
		correctWasm    = getExampleFile(t, "./test_files/circuit.wasm")
		correctWasm2   = getExampleFile(t, "./test_files/circuit2.wasm")
		correctInputs  = getExampleFile(t, "./test_files/inputs.json")
		correctInputs2 = getExampleFile(t, "./test_files/inputs2.json")
	)

	// Empty and first set of valid parameters
	var emptyWasm, emptyInputs = []byte{}, []byte{}
	if _, err := calcWitness(emptyWasm, correctInputs); err == nil {
		t.Error("expected error raised by wrong circuit provided, got nil")
	} else if _, err = calcWitness(correctWasm, emptyInputs); err == nil {
		t.Error("expected error raised by wrong inputs provided, got nil")
	} else if wtns, err := calcWitness(correctWasm, correctInputs); err != nil {
		t.Errorf("expected nil, got error %v", err)
	} else if len(wtns) == 0 {
		t.Error("expected not empty witness result, got empty []byte")
	}

	// Wrong and second set of valid parameters
	if _, err := calcWitness(correctWasm, correctInputs2); err == nil {
		t.Error("expected error raised by wrong inputs (2) for the provided circuit (1), got nil")
	} else if _, err = calcWitness(correctWasm2, correctInputs); err == nil {
		t.Error("expected error raised by wrong inputs (1) for the provided circuit (2), got nil")
	} else if wtns, err := calcWitness(correctWasm2, correctInputs2); err != nil {
		t.Errorf("expected nil, got error %v", err)
	} else if len(wtns) == 0 {
		t.Error("expected not empty witness result, got empty []byte")
	}
}

func TestProve(t *testing.T) {
	var (
		wasm        = getExampleFile(t, "./test_files/circuit.wasm")
		wasm2       = getExampleFile(t, "./test_files/circuit2.wasm")
		inputs      = getExampleFile(t, "./test_files/inputs.json")
		inputs2     = getExampleFile(t, "./test_files/inputs2.json")
		zkey        = getExampleFile(t, "./test_files/proving_key.zkey")
		zkey2       = getExampleFile(t, "./test_files/proving_key2.zkey")
		pubSignals  = getExampleFile(t, "./test_files/public_signals.json")
		pubSignals2 = getExampleFile(t, "./test_files/public_signals2.json")
	)

	// Empty and valid parameters
	var emptyZkey = []byte{}
	if _, _, err := Prove(emptyZkey, wasm, inputs); err == nil {
		t.Error("expected error raised by empty zkey provided, got nil")
	} else if _, resPubSignals, err := Prove(zkey, wasm, inputs); err != nil {
		t.Errorf("expected nil, got error %v", err)
	} else if !reflect.DeepEqual(pubSignals, resPubSignals) {
		t.Errorf("expected %v, got %v", string(pubSignals), string(resPubSignals))
	}

	// Second set of valid parameters
	if _, resPubSignals, err := Prove(zkey2, wasm2, inputs2); err != nil {
		t.Errorf("expected nil, got error %v", err)
	} else if !reflect.DeepEqual(pubSignals2, resPubSignals) {
		t.Errorf("expected %v, got %v", string(pubSignals), string(resPubSignals))
	}
}

func TestVerify(t *testing.T) {
	var (
		wasm   = getExampleFile(t, "./test_files/circuit.wasm")
		inputs = getExampleFile(t, "./test_files/inputs.json")
		zkey   = getExampleFile(t, "./test_files/proving_key.zkey")
		vkey   = getExampleFile(t, "./test_files/verification_key.json")
		zkey2  = getExampleFile(t, "./test_files/proving_key2.zkey")
		vkey2  = getExampleFile(t, "./test_files/verification_key2.json")
	)

	// Check a valid case
	var validProofData, validPublicSignals, _ = Prove(zkey, wasm, inputs)
	if err := Verify(vkey, validProofData, validPublicSignals); err != nil {
		t.Errorf("expected got nil (valid proof), got %v", err)
	}

	// Check a invalid case with empty parameters
	var emptyVkey, emptyProofData, emptyPublicSignals = []byte{}, []byte{}, []byte{}
	if err := Verify(emptyVkey, validProofData, validPublicSignals); err == nil {
		t.Errorf("expected error raised by empty verification key, got error %v", err)
	} else if err = Verify(vkey, emptyProofData, validPublicSignals); err == nil {
		t.Errorf("expected error raised by empty proof, got error %v", err)
	} else if err = Verify(vkey, validProofData, emptyPublicSignals); err == nil {
		t.Errorf("expected error raised by empty public signals, got error %v", err)
	}

	// Check a proof generated with a different zkey
	var invalidProofData, invalidPublicSignals, _ = Prove(zkey2, wasm, inputs)
	if err := Verify(vkey, invalidProofData, invalidPublicSignals); err == nil {
		t.Errorf("expected error raised by a zkey not associated with the circuit or the vkey, got error %v", err)
	} else if err := Verify(vkey2, invalidProofData, invalidPublicSignals); err == nil {
		t.Errorf("expected error raised by a zkey not associated with the circuit but associated with the vkey, got error %v", err)
	}
}
