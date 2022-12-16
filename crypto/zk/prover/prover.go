// prover package abstracts the logic and types of go-rapidsnark
// (https://github.com/iden3/go-rapidsnark) to support basic operations for the
// rest of vocdoni-node project. It returns custom errors with some more
// information, trying to clarify the original workflow and standardizing the
// inputs/outputs types.
package prover

import (
	"encoding/json"
	"fmt"

	"github.com/iden3/go-rapidsnark/prover"
	"github.com/iden3/go-rapidsnark/types"
	"github.com/iden3/go-rapidsnark/verifier"
	"github.com/iden3/go-rapidsnark/witness"
)

var (
	ErrParsingWitness  = fmt.Errorf("error parsing provided circuit inputs, it must be a not empty unmarshalled bytes of a json")
	ErrInitWitnessCalc = fmt.Errorf("error parsing circuit wasm during calculator instance")
	ErrWitnessCalc     = fmt.Errorf("error during witness calculation")
	ErrProofGen        = fmt.Errorf("error during zksnark proof generation")
	ErrParseProofData  = fmt.Errorf("error parsing the proof provided, it must be a valid json, check https://github.com/iden3/go-rapidsnark/blob/73d5784d2aa791dd6646142b0017dbef97240f57/types/proof.go")
	ErrParsePubSignals = fmt.Errorf("error during zksnark proof generation, it must be a json with array of strings")
	ErrVerifyProof     = fmt.Errorf("error during zksnark verification")
)

// calcWitness perform the witness calculation using go-rapidsnark library based
// on wasm version of the circuit and inputs provided. To provide the arguments
// into the correct way, just read the content of wasm binary and inputs JSON
// files.
func calcWitness(wasmBytes, inputsBytes []byte) (res []byte, panicErr error) {
	// If the inputs are empty or bad formatted, it raises a panic. To avoid it,
	// catch the panic and return an error instead.
	defer func() {
		if p := recover(); p != nil {
			panicErr = ErrParsingWitness
		}
	}()

	// Parse the []byte inputs into a map using the go-rapidsnark/witness
	// ParseInputs function. Raw JSON Unmarshal result raise an error during
	// witness calculation.
	inputs, err := witness.ParseInputs(inputsBytes)
	if err != nil {
		return nil, ErrParsingWitness
	}

	// Instances a go-rapidsnark/witness calculator with the provided wasm []byte
	calculator, err := witness.NewCircom2WitnessCalculator(wasmBytes, true)
	if err != nil {
		return nil, ErrInitWitnessCalc
	}

	// Perform the witness calculation
	wtns, err := calculator.CalculateWTNSBin(inputs, true)
	if err != nil {
		return nil, ErrWitnessCalc
	}

	return wtns, nil
}

// Prove function generates a verifiable proof of the execution of the circuit
// for the input signals using the proving key provided. All the arguments are
// slices of bytes with the data read from the generated files by Circom (wasm
// circuit) and SnarkJS (proving zkey). It returns the verifiable proof of the
// execution with the public signals associated.
func Prove(zKey, wasm, inputs []byte) ([]byte, []byte, error) {
	// Calculate the witness calling internal function calcWitness with the
	// provided wasm and inputs.
	wtns, err := calcWitness(wasm, inputs)
	if err != nil {
		return nil, nil, err
	}

	// Generate the proof and public signals with the witness calculated and the
	// proving zkey provided.
	strProof, strPubSignals, err := prover.Groth16ProverRaw(zKey, wtns)
	if err != nil {
		return nil, nil, ErrProofGen
	}

	// Return the proof and public signals as slices of bytes
	return []byte(strProof), []byte(strPubSignals), nil
}

// Verify function performs a verification of the provided proof and public
// signals. It receives the verification key, the proof and the public signals
// into slices of bytes, result of origin files read. It returns an error if
// something fails or nil if the verification was ok.
func Verify(vKey, proofData, pubSignals []byte) error {
	// Instance a required proof struct to fill it then
	proof := types.ZKProof{
		Proof:      &types.ProofData{},
		PubSignals: []string{},
	}

	// Read proof and public signals as regular JSON into the instanced proof
	// struct and try to verify it with go-rapidsnark/verifier.
	if err := json.Unmarshal(proofData, &proof.Proof); err != nil {
		return ErrParseProofData
	} else if err = json.Unmarshal(pubSignals, &proof.PubSignals); err != nil {
		return ErrParsePubSignals
	} else if err = verifier.VerifyGroth16(proof, vKey); err != nil {
		return ErrVerifyProof
	}

	// Return nil if everything was ok.
	return nil
}
