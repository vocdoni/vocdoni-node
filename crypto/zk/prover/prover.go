// prover package abstracts the logic and types of go-rapidsnark
// (https://github.com/iden3/go-rapidsnark) to support basic operations for the
// rest of vocdoni-node project. It returns custom errors with some more
// information, trying to clarify the original workflow and standardizing the
// inputs/outputs types.
package prover

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/iden3/go-rapidsnark/prover"
	"github.com/iden3/go-rapidsnark/types"
	"github.com/iden3/go-rapidsnark/verifier"
	"github.com/iden3/go-rapidsnark/witness"
)

// TODO: Refactor the error handling to include the trace of the original error
// into the error returned.
var (
	ErrPublicSignalFormat = fmt.Errorf("invalid proof public signals format")
	ErrParsingWeight      = fmt.Errorf("error parsing proof weight string to big.Int")
	ErrParsingNullifier   = fmt.Errorf("error parsing proof nullifier string to big.Int")
	ErrParsingWitness     = fmt.Errorf("error parsing provided circuit inputs, it must be a not empty marshalled bytes of a json")
	ErrInitWitnessCalc    = fmt.Errorf("error parsing circuit wasm during calculator instance")
	ErrWitnessCalc        = fmt.Errorf("error during witness calculation")
	ErrProofGen           = fmt.Errorf("error during zksnark proof generation")
	ErrParseProofData     = fmt.Errorf("error parsing the proof provided, it must be a valid json, check https://github.com/iden3/go-rapidsnark/blob/73d5784d2aa791dd6646142b0017dbef97240f57/types/proof.go")
	ErrParsePubSignals    = fmt.Errorf("error during zksnark public signals proof generation, it must be a json with array of strings")
	ErrEncodingProof      = fmt.Errorf("error encoding prove result into a proof struct")
	ErrDecodingProof      = fmt.Errorf("error decoding prove as []byte")
	ErrVerifyProof        = fmt.Errorf("error during zksnark verification")
)

// ProofData struct contains the calculated parameters of a Proof. It allows to
// encode and decode go-rapidsnark inputs and outputs easily.
type ProofData struct {
	A []string   `json:"pi_a"`
	B [][]string `json:"pi_b"`
	C []string   `json:"pi_c"`
}

// Proof struct wraps the ProofData struct and its associated public signals.
// Contains all the required information to perform a proof verification.
type Proof struct {
	Data       ProofData `json:"data"`
	PubSignals []string  `json:"pubSignals"`
}

// ParseProof encodes the provided proof data and public signals into a Proof
// struct, performing an unmarshal operation over them. Returns an error if
// something is wrong.
func ParseProof(proofData, pubSignals []byte) (*Proof, error) {
	data := ProofData{}
	if err := json.Unmarshal(proofData, &data); err != nil {
		return nil, ErrEncodingProof
	}

	signals := []string{}
	if err := json.Unmarshal(pubSignals, &signals); err != nil {
		return nil, ErrEncodingProof
	}
	return &Proof{Data: data, PubSignals: signals}, nil
}

// Bytes returns the current Proof struct parameters Data and PubSignals as
// []byte. It returns an error if something fails.
func (p *Proof) Bytes() ([]byte, []byte, error) {
	proofData, err := json.Marshal(p.Data)
	if err != nil {
		return nil, nil, ErrDecodingProof
	}

	pubSignals, err := json.Marshal(p.PubSignals)
	if err != nil {
		return nil, nil, ErrDecodingProof
	}

	return proofData, pubSignals, nil
}

// Weight decodes the vote weight value from the current proof public signals
// and return it as a big.Int.
func (p *Proof) Weight() (*big.Int, error) {
	// Check if the current proof contains public signals and it contains the
	// correct number of positions.
	if p.PubSignals == nil || len(p.PubSignals) < 5 {
		return nil, ErrPublicSignalFormat
	}
	// Get the weight from the fourth public signal of the proof
	strWeight := p.PubSignals[4]
	// Parse it into a big.Int
	weight, ok := new(big.Int).SetString(strWeight, 10)
	if !ok {
		return nil, ErrParsingNullifier
	}
	return weight, nil
}

// Nullifier decodes the vote nullifier value from the current proof public
// signals and return it as a big.Int
func (p *Proof) Nullifier() (*big.Int, error) {
	if p.PubSignals == nil || len(p.PubSignals) < 5 {
		return nil, ErrPublicSignalFormat
	}
	// Get the nullifier from the fourth public signal of the proof
	strNullifier := p.PubSignals[3]
	// Parse it into a big.Int
	nullifier, ok := new(big.Int).SetString(strNullifier, 10)
	if !ok {
		return nil, ErrParsingWeight
	}
	return nullifier, nil
}

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

	// Instances a go-rapidsnark/witness calculator with the provided wasm
	// []byte
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

// Prove generates a verifiable proof of the execution of the circuit for the
// input signals using the proving key provided. All the arguments are slices of
// bytes with the data read from the generated files by Circom (wasm circuit)
// and SnarkJS (proving zkey). It returns the verifiable proof of the execution
// with the public signals associated or an error if something fails.
func Prove(zKey, wasm, inputs []byte) (*Proof, error) {
	// Calculate the witness calling internal function calcWitness with the
	// provided wasm and inputs.
	wtns, err := calcWitness(wasm, inputs)
	if err != nil {
		return nil, err
	}

	// Generate the proof and public signals with the witness calculated and the
	// proving zkey provided.
	strProofData, strPubSignals, err := prover.Groth16ProverRaw(zKey, wtns)
	if err != nil {
		return nil, ErrProofGen
	}

	// Parse the components generated into a prover.Proof struct
	proof, err := ParseProof([]byte(strProofData), []byte(strPubSignals))
	if err != nil {
		return nil, err
	}
	// Return the proof and public signals as slices of bytes
	return proof, nil
}

// Verify performs a verification of the provided proof and its public signals.
// It receives the verification key and returns an error if something fails or
// nil if the verification was ok.
func (p *Proof) Verify(vKey []byte) error {
	proof := types.ZKProof{
		Proof: &types.ProofData{
			A: p.Data.A,
			B: p.Data.B,
			C: p.Data.C,
		},
		PubSignals: p.PubSignals,
	}
	// Try to verify the provided proof with go-rapidsnark/verifier.
	if err := verifier.VerifyGroth16(proof, vKey); err != nil {
		return ErrVerifyProof
	}

	// Return nil if everything is ok.
	return nil
}
