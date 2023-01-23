package circuit

import (
	"encoding/hex"
	"log"
	"strings"
)

var DefaultCircuitConfigurationTag = "dev"

// ZkCircuitConfig defines the configuration of the files to be downloaded
type ZkCircuitConfig struct {
	// URI defines the URI from where to download the files
	URI string `json:"uri"`
	// CircuitPath defines the path from where the files are downloaded
	CircuitPath string `json:"circuitPath"`
	// Parameters used for the circuit build
	Parameters int `json:"parameters"`
	Levels     int `json:"levels"`
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
	"bizono": {
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
		CircuitPath:         "zkcensusproof/dev/65536",
		Parameters:          65536, // 2^Levels
		Levels:              16,    // ZkCircuit number of levels
		LocalDir:            "zkCircuits",
		ProvingKeyHash:      hexToBytes("0xb7fb6f74ecf56e41de103e679c76c45a1bde99e2203b2ab6928396020f4d4ab6"),
		VerificationKeyHash: hexToBytes("0x50029154e81a2078eff83751454bb3ece2cf9391103cc17306d47f7d4461b0b6"),
		WasmHash:            hexToBytes("0x1d975d68220d1f10bd54e2f53ea9526ce8f916efb15a2079edc3db9403a78278"),
	},
	"dev": {
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-franchise-proof-circuit/feature/merging_repos_and_new_tests",
		CircuitPath:         "artifacts/zkCensus/dev/16",
		Parameters:          65536, // 2^Levels
		Levels:              16,    // ZkCircuit number of levels
		LocalDir:            "zkCircuits",
		ProvingKeyHash:      hexToBytes("0x96c318c8f75a47069b5d4b22a5d782b79319f666e02f11e49d620d75674f9930"),
		VerificationKeyHash: hexToBytes("0x591cec6d8ef71a6b45b495acba413d44d263557e48194428ab706bedf14624cc"),
		WasmHash:            hexToBytes("0xc1bad9e7ff7f6700ea4a38956168b2114328c7e12a9fee1f0b05f25a0f62e3d2"),
	},
	"stage": {
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
		CircuitPath:         "zkcensusproof/dev/1024",
		Parameters:          1024, // 2^Levels
		Levels:              10,   // ZkCircuit number of levels
		LocalDir:            "./circuits",
		ProvingKeyHash:      hexToBytes("0x1cd0c9225210700d4d6307493bbe5f98554e29339daba6d9bd08a4e0e78df443"),
		VerificationKeyHash: hexToBytes("0xaed892ff98ab37b877cfcb678cb5f48f1be9d09dbbaf74b5877f46b54d10f9ad"),
		WasmHash:            hexToBytes("0x61b40e11ece8de3fbfaf27dbd984e0e0b1fa05ee72d4faa0c2be06c1d7a9b845"),
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
