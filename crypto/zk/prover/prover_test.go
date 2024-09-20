package prover

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"testing"

	"go.vocdoni.io/dvote/util"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
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

	t.Logf("%+v", prettyPrint(proof))

	gnarkproof := ProofToGnarkProof(proof)

	_ = sp1WitnessInput(inputs)

	sp1proof := GnarkProofToSP1Proof(gnarkproof, WitnessInput{
		"1234", // this should be the hash of the verification key (in gnark binary format) vk.hash_bn254().as_canonical_biguint()
		"5678", // and this the hash of the SP1PublicValues public_values.hash_bn254()
	})

	t.Logf("%+v", prettyPrint(sp1proof))
}

func sp1WitnessInput(inputs []byte) WitnessInput {
	witnessInput := WitnessInput{}
	gnarkVK, err := parseVKForGnark(vkey)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v", prettyPrint(gnarkVK))

	witnessInput.VkeyHash = hashBN254(gnarkVK)            // TODO
	witnessInput.CommitedValuesDigest = hashBN254(inputs) // TODO
	return witnessInput
}

func hashBN254(any) string {
	return "deadcode"
}

func prettyPrint(v interface{}) string {
	// Convert the struct to a pretty JSON format
	prettyJSON, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(prettyJSON)
}

// Function to convert a hex string to a big.Int
func hexToBigInt(hexStr string) (*big.Int, error) {
	data, err := hex.DecodeString(util.TrimHex(hexStr))
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(data), nil
}

func ProofToGnarkProof(proof *Proof) *groth16_bn254.Proof {
	gnarkproof, err := convertToGnarkProof(proof.Data)
	if err != nil {
		log.Fatal(err)
	}
	return gnarkproof
}

// Convert ProofData to gnark-compatible Groth16 proof
func convertToGnarkProof(pd ProofData) (*groth16_bn254.Proof, error) {
	gnarkProof := &groth16_bn254.Proof{}

	// Convert pi_a (G1) - Point A
	aXBigInt, err := stringToBigInt(pd.A[0])
	if err != nil {
		return gnarkProof, err
	}
	aYBigInt, err := stringToBigInt(pd.A[1])
	if err != nil {
		return gnarkProof, err
	}

	// Initialize fr.Element for A point
	var aX, aY fp.Element
	aX.SetBigInt(aXBigInt)
	aY.SetBigInt(aYBigInt)

	gnarkProof.Ar = bn254.G1Affine{
		X: aX,
		Y: aY,
	}

	// Convert pi_b (G2) - Point B
	bX0BigInt, err := stringToBigInt(pd.B[0][0])
	if err != nil {
		return gnarkProof, err
	}
	bX1BigInt, err := stringToBigInt(pd.B[0][1])
	if err != nil {
		return gnarkProof, err
	}
	bY0BigInt, err := stringToBigInt(pd.B[1][0])
	if err != nil {
		return gnarkProof, err
	}
	bY1BigInt, err := stringToBigInt(pd.B[1][1])
	if err != nil {
		return gnarkProof, err
	}

	// Initialize fr.Element for B points
	var bX0, bX1, bY0, bY1 fp.Element
	bX0.SetBigInt(bX0BigInt)
	bX1.SetBigInt(bX1BigInt)
	bY0.SetBigInt(bY0BigInt)
	bY1.SetBigInt(bY1BigInt)

	// Construct the G2 element for Bs (G2Affine expects fptower.E2 for both X and Y)
	gnarkProof.Bs.X.A0 = bX0
	gnarkProof.Bs.X.A1 = bX1
	gnarkProof.Bs.Y.A0 = bY0
	gnarkProof.Bs.Y.A1 = bY1

	// Convert pi_c (G1) - Point C
	cXBigInt, err := stringToBigInt(pd.C[0])
	if err != nil {
		return gnarkProof, err
	}
	cYBigInt, err := stringToBigInt(pd.C[1])
	if err != nil {
		return gnarkProof, err
	}

	// Initialize fr.Element for C point
	var cX, cY fp.Element
	cX.SetBigInt(cXBigInt)
	cY.SetBigInt(cYBigInt)

	gnarkProof.Krs.X = cX
	gnarkProof.Krs.Y = cY

	return gnarkProof, nil
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

func GnarkProofToSP1Proof(proof *groth16_bn254.Proof, witnessInput WitnessInput) SP1Proof {
	var publicInputs [2]string
	publicInputs[0] = witnessInput.VkeyHash
	publicInputs[1] = witnessInput.CommitedValuesDigest

	encodedProof := proof.MarshalSolidity()

	return SP1Proof{
		PublicInputs: publicInputs,
		EncodedProof: hex.EncodeToString(encodedProof),
		// RawProof:     hex.EncodeToString(proofBytes), // this field is uninteresting AFAIU
	}
}

// Verification Key

// vkJSON is the format of the verification key from go-rapidsnark, copypasta from
// ~/go/pkg/mod/github.com/iden3/go-rapidsnark/verifier@v0.0.3/parser.go
type vkJSON struct {
	Alpha []string   `json:"vk_alpha_1"`
	Beta  [][]string `json:"vk_beta_2"`
	Gamma [][]string `json:"vk_gamma_2"`
	Delta [][]string `json:"vk_delta_2"`
	IC    [][]string `json:"IC"`
}

// Converts hexadecimal string to a field element (fr.Element)
func hexToFieldElement(hexStr string) (fp.Element, error) {
	var elem fp.Element
	bigInt, success := new(big.Int).SetString(hexStr, 10) // assuming decimal strings
	if !success {
		return elem, fmt.Errorf("invalid field element: %s", hexStr)
	}
	elem.SetBigInt(bigInt)
	return elem, nil
}

// Converts a slice of string to a G1 point
func stringToG1(coords []string) (bn254.G1Affine, error) {
	var p bn254.G1Affine
	if len(coords) != 3 {
		return p, fmt.Errorf("invalid G1 coordinates: %v", coords)
	}
	x, err := hexToFieldElement(coords[0])
	if err != nil {
		return p, err
	}
	y, err := hexToFieldElement(coords[1])
	if err != nil {
		return p, err
	}
	p.X = x
	p.Y = y
	return p, nil
}

// Converts a slice of string to a G2 point
func stringToG2(coords [][]string) (bn254.G2Affine, error) {
	var p bn254.G2Affine
	if len(coords) != 3 {
		return p, fmt.Errorf("invalid G2 coordinates")
	}

	// Convert X (A0, A1)
	xA0, err := hexToFieldElement(coords[0][0])
	if err != nil {
		return p, err
	}
	xA1, err := hexToFieldElement(coords[0][1])
	if err != nil {
		return p, err
	}

	// Convert Y (A0, A1)
	yA0, err := hexToFieldElement(coords[1][0])
	if err != nil {
		return p, err
	}
	yA1, err := hexToFieldElement(coords[1][1])
	if err != nil {
		return p, err
	}

	p.X.A0, p.X.A1 = xA0, xA1
	p.Y.A0, p.Y.A1 = yA0, yA1
	return p, nil
}

// Parses the vkJSON into gnark's groth16.VerifyingKey
func parseVKForGnark(vkjson []byte) (*groth16_bn254.VerifyingKey, error) {
	var vkStr vkJSON
	err := json.Unmarshal(vkjson, &vkStr)
	if err != nil {
		return nil, fmt.Errorf("Error parsing verification key JSON:", err)
	}

	vk := new(groth16_bn254.VerifyingKey)

	// Parse Alpha (G1 point)
	alpha, err := stringToG1(vkStr.Alpha)
	if err != nil {
		return nil, err
	}
	vk.G1.Alpha = alpha

	// Parse Beta (G2 point)
	beta, err := stringToG2(vkStr.Beta)
	if err != nil {
		return nil, err
	}
	vk.G2.Beta = beta

	// Parse Gamma (G2 point)
	gamma, err := stringToG2(vkStr.Gamma)
	if err != nil {
		return nil, err
	}
	vk.G2.Gamma = gamma

	// Parse Delta (G2 point)
	delta, err := stringToG2(vkStr.Delta)
	if err != nil {
		return nil, err
	}
	vk.G2.Delta = delta

	// Parse IC (G1 points array)
	for _, icCoords := range vkStr.IC {
		icPoint, err := stringToG1(icCoords)
		if err != nil {
			return nil, err
		}
		vk.G1.K = append(vk.G1.K, icPoint)
	}

	return vk, nil
}
