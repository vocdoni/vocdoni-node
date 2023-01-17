package circuit

import (
	"encoding/hex"
	"log"
	"strings"
)

const ZKCircuitIndex int32 = 1

// ZkCircuitConfig defines the configuration of the files to be downloaded
type ZkCircuitConfig struct {
	// URI defines the URI from where to download the files
	URI string `json:"uri"`
	// CircuitPath defines the path from where the files are downloaded
	CircuitPath string `json:"circuitPath"`
	// Parameters used for the circuit build
	Parameters []int64 `json:"parameters"`
	Levels     int     `json:"levels"`
	// LocalDir defines in which directory will be the files
	// downloaded, under that directory it will follow the CircuitPath
	// directories structure
	LocalDir string `json:"localDir"`

	// WasmHash contains the expected hash for the file filenameWasm
	WasmHash []byte `json:"wasmHash"` // circuit.wasm
	// ProvingKeyHash contains the expected hash for the file filenameZKey
	ProvingKeyHash []byte `json:"zKeyHash"` // proving_key.zkey
	// VerificationKeyHash contains the expected hash for the file filenameVK
	VerificationKeyHash []byte `json:"vKHash"` // verification_key.json
}

// CircuitsConfiguration stores the relation between the different vochain nets
// and the associated circuit configuration. Any circuit configuration must have
// the remote and local location of the circuits artifacts and their metadata
// such as artifacts hash or the number of parameters.
var CircuitsConfigurations = map[string]ZkCircuitConfig{
	"dev": { // index: 1, size: 65k
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-franchise-proof-circuit/feature/merging_repos_and_new_tests",
		CircuitPath:         "artifacts/zkCensus/dev/16",
		Parameters:          []int64{65536}, // 2^16
		Levels:              16,
		LocalDir:            "zkCircuits",
		ProvingKeyHash:      hexToBytes("0x96c318c8f75a47069b5d4b22a5d782b79319f666e02f11e49d620d75674f9930"),
		VerificationKeyHash: hexToBytes("0x591cec6d8ef71a6b45b495acba413d44d263557e48194428ab706bedf14624cc"),
		WasmHash:            hexToBytes("0xc1bad9e7ff7f6700ea4a38956168b2114328c7e12a9fee1f0b05f25a0f62e3d2"),
	},
}

// hexToBytes parses a hex string and returns the byte array from it. Warning,
// in case of error it will panic.
func hexToBytes(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		log.Fatalf("Error decoding hex string %s: %s", s, err)
	}
	return b
}
