// prover package abstracts the logic and types of go-rapidsnark
// (https://github.com/iden3/go-rapidsnark) to support basic operations for the
// rest of vocdoni-node project. It returns custom errors with some more
// information, trying to clarify the original workflow and standardizing the
// inputs/outputs types.
package prover

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/iden3/go-rapidsnark/prover"
	"github.com/iden3/go-rapidsnark/types"
	"github.com/iden3/go-rapidsnark/verifier"
	"github.com/iden3/go-rapidsnark/witness"
)

// calcWitness perform the witness calculation using go-rapidsnark library based
// on wasm version of the circuit and inputs provided. To provide the arguments
// into the correct way, just read the content of wasm binary and inputs JSON
// files.
func calcWitness(wasmBytes, inputsBytes []byte) (wtns []byte, err error) {
	// Parse the []byte inputs into a map using the go-rapidsnark/witness
	// ParseInputs function. Raw JSON Unmarshal result raise an error during
	// witness calculation.
	var inputs map[string]interface{}
	if inputs, err = witness.ParseInputs(inputsBytes); err != nil {
		var msg = fmt.Sprintf("error parsing provided circuit inputs, it must be the unmarshalled bytes of a json: %v", err)
		return nil, errors.New(msg)
	}

	// Instance a go-rapidsnark/witness calculator with the provided wasm []byte
	var calc *witness.Circom2WitnessCalculator
	if calc, err = witness.NewCircom2WitnessCalculator(wasmBytes, true); err != nil {
		var msg = fmt.Sprintf("error parsing circuit wasm during calculator instance: %v", err)
		return nil, errors.New(msg)
	}

	// Perform the witness calculation
	if wtns, err = calc.CalculateWTNSBin(inputs, true); err != nil {
		var msg = fmt.Sprintf("error during witness calculation: %v", err)
		return nil, errors.New(msg)
	}

	return
}

// Prove function generates a verificable proof of the execution of the circuit
// for the inputs signals using the proving key provided. All the arguments are
// slices of bytes with the data read from the generated files by Circom (wasm
// circuit) and SnarkJS (proving zkey). It returns the verificable proof of the
// execution with the public signals associated.
func Prove(zKey, wasm, inputs []byte) (proof, pubSignals []byte, err error) {
	// Calculate the witness calling internal function calcWitness with the
	// provided wasm and inputs.
	var wtns []byte
	if wtns, err = calcWitness(wasm, inputs); err != nil {
		return
	}

	// Generate the proof and public signals with the witness calculated and the
	// proving zkey provided.
	var strProof, strPubSignals string
	if strProof, strPubSignals, err = prover.Groth16ProverRaw(zKey, wtns); err != nil {
		var msg = fmt.Sprintf("error during zksnark proof generation: %v", err)
		err = errors.New(msg)
		return
	}

	// Return the proof and public signals as slices of bytes
	return []byte(strProof), []byte(strPubSignals), nil
}

// Verify function perform a verification of the provided proof and public
// signals. It receives the verification key, the proof and the public signals
// into slices of bytes, result of origin files read. It returns and error if
// something fails or nil if the verification was ok.
func Verify(vKey, proofData, pubSignals []byte) error {
	// Instance a required proof struct to fill it then
	var proof = types.ZKProof{
		Proof:      &types.ProofData{},
		PubSignals: []string{},
	}

	// Read proof and public signals as regular JSON into the instanced proof
	// struct and try to verify it with go-rapidsnark/verifier.
	if err := json.Unmarshal(proofData, &proof.Proof); err != nil {
		var msg = fmt.Sprintf("error parsing the proof provided, it must be a valid json, check https://github.com/iden3/go-rapidsnark/blob/73d5784d2aa791dd6646142b0017dbef97240f57/types/proof.go: %v", err)
		return errors.New(msg)
	} else if err = json.Unmarshal(pubSignals, &proof.PubSignals); err != nil {
		var msg = fmt.Sprintf("error during zksnark proof generation, it must be a json with array of strings: %v", err)
		return errors.New(msg)
	} else if err = verifier.VerifyGroth16(proof, vKey); err != nil {
		var msg = fmt.Sprintf("error during zksnark verification: %v", err)
		return errors.New(msg)
	}

	// Return nil if everything was ok.
	return nil
}
