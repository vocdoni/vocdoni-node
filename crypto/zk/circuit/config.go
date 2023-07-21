package circuit

import (
	"encoding/hex"
	"log"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

// DefaultCircuitConfigurationTag constant contains the tag value that points
// to the default ZkSnark circuit configuration. It ensures that at least one
// circuit configuration is available so the configuration referred by this tag
// must be defined.
const DefaultCircuitConfigurationTag = "dev"

// ZkCircuitConfig defines the configuration of the files to be downloaded
type ZkCircuitConfig struct {
	// URI defines the URI from where to download the files
	URI string `json:"uri"`
	// CircuitPath defines the path from where the files are downloaded.
	// Locally, they will be cached inside circuit.BaseDir path,
	// under that directory it will follow the CircuitPath dir structure
	CircuitPath string `json:"circuitPath"`
	// Levels refers the number of levels that the merkle tree associated to the
	// current circuit configuration artifacts has
	Levels int `json:"levels"`
	// ProvingKeyHash contains the expected hash for the file filenameZKey
	ProvingKeyHash types.HexBytes `json:"zKeyHash"`
	// FilenameProvingKey defines the name of the file of the circom ProvingKey
	ProvingKeyFilename string `json:"zKeyFilename"` // proving_key.zkey
	// VerificationKeyHash contains the expected hash for the file filenameVK
	VerificationKeyHash types.HexBytes `json:"vKeyHash"`
	// FilenameVerificationKey defines the name of the file of the circom
	// VerificationKey
	VerificationKeyFilename string `json:"vKeyFilename"` // verification_key.json
	// WasmHash contains the expected hash for the file filenameWasm
	WasmHash types.HexBytes `json:"wasmHash"`
	// FilenameWasm defines the name of the file of the circuit wasm compiled
	// version
	WasmFilename string `json:"wasmFilename"` // circuit.wasm
}

// KeySize returns the maximum number of bytes of a leaf key according to the
// number of levels of the current circuit (nBytes = nLevels / 8).
func (config ZkCircuitConfig) KeySize() int {
	return config.Levels / 8
}

// CircuitsConfigurations stores the relation between the different vochain nets
// and the associated circuit configuration. Any circuit configuration must have
// the remote and local location of the circuits artifacts and their metadata
// such as artifacts hash or the number of parameters.
var CircuitsConfigurations = map[string]ZkCircuitConfig{
	"dev": {
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-franchise-proof-circuit/feature/new-circuit",
		CircuitPath:             "artifacts/zkCensus/dev/160",
		Levels:                  160, // ZkCircuit number of levels
		ProvingKeyHash:          hexToBytes("0xb4487df5f2cb2834f2bf3680dd92516df5ace950fe7379730dc000e6a06fb7db"),
		ProvingKeyFilename:      "proving_key.zkey",
		VerificationKeyHash:     hexToBytes("0x16d2b61bccc06197e04c2dc9a1055998ccdbd34fa0f286b195ddca04e087daa0"),
		VerificationKeyFilename: "verification_key.json",
		WasmHash:                hexToBytes("0x5502eae8c658adbdc609cd7da68fa14847bb2bc0665d4cdb4f988a33c7619518"),
		WasmFilename:            "circuit.wasm",
	},
}

// GetCircuitConfiguration returns the circuit configuration associated with the
// provided tag or gets the default one.
func GetCircuitConfiguration(configTag string) ZkCircuitConfig {
	if conf, ok := CircuitsConfigurations[configTag]; ok {
		return conf
	}
	return CircuitsConfigurations[DefaultCircuitConfigurationTag]
}

// hexToBytes parses a hex string and returns the byte array from it. Warning,
// in case of error it will panic.
func hexToBytes(s string) []byte {
	b, err := hex.DecodeString(util.TrimHex(s))
	if err != nil {
		log.Fatalf("Error decoding hex string %s: %s", s, err)
	}
	return b
}
