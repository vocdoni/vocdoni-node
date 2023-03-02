package circuit

import (
	"encoding/hex"
	"log"
	"strings"
)

// DefaultCircuitConfigurationTag constant contains the tag value that points
// to the default ZkSnark circuit configuration. It ensures that at least one
// circuit configuration is available so the configuration refered by this tag
// must be defined.
const DefaultCircuitConfigurationTag = "dev"

// ZkCircuitConfig defines the configuration of the files to be downloaded
type ZkCircuitConfig struct {
	// URI defines the URI from where to download the files
	URI string `json:"uri"`
	// CircuitPath defines the path from where the files are downloaded
	CircuitPath string `json:"circuitPath"`
	// Levels refers the number of levels that the merkle tree associated to the
	// current circuit configuration artifacts has
	Levels int `json:"levels"`
	// LocalDir defines in which directory will be the files
	// downloaded, under that directory it will follow the CircuitPath
	// directories structure
	LocalDir string `json:"localDir,omitempty"`
	// ProvingKeyHash contains the expected hash for the file filenameZKey
	ProvingKeyHash []byte `json:"zKeyHash"`
	// FilenameProvingKey defines the name of the file of the circom ProvingKey
	ProvingKeyFilename string `json:"zKeyFilename"` // proving_key.zkey
	// VerificationKeyHash contains the expected hash for the file filenameVK
	VerificationKeyHash []byte `json:"vKeyHash"`
	// FilenameVerificationKey defines the name of the file of the circom
	// VerificationKey
	VerificationKeyFilename string `json:"vKeyFilename"` // verification_key.json
	// WasmHash contains the expected hash for the file filenameWasm
	WasmHash []byte `json:"wasmHash"`
	// FilenameWasm defines the name of the file of the circuit wasm compiled
	// version
	WasmFilename string `json:"wasmFilename"` // circuit.wasm
}

// KeySize returns the maximun number of bytes of a leaf key according to the
// number of levels of the current circuit (nBytes = nLevels / 8).
func (config ZkCircuitConfig) KeySize() int {
	return config.Levels / 8
}

// CircuitsConfiguration stores the relation between the different vochain nets
// and the associated circuit configuration. Any circuit configuration must have
// the remote and local location of the circuits artifacts and their metadata
// such as artifacts hash or the number of parameters.
var CircuitsConfigurations = map[string]ZkCircuitConfig{
	// TODO: set up the circuit URI's to the branch that supports customWeight
	"dev": {
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-franchise-proof-circuit/master",
		CircuitPath:             "artifacts/zkCensus/dev/160",
		Levels:                  160, // ZkCircuit number of levels
		LocalDir:                "zkCircuits",
		ProvingKeyHash:          hexToBytes("0x29b5d4ebfe673794fea376922355de538c52423689098f1e10d10e92987fbef6"),
		ProvingKeyFilename:      "proving_key.zkey",
		VerificationKeyHash:     hexToBytes("0x1d6818b479f80211feb19a68fb9dff78e94d0aa0e32e1df8e6ede61beb75c0ce"),
		VerificationKeyFilename: "verification_key.json",
		WasmHash:                hexToBytes("0x0b25bd8ac0861f3d3843fc9a9c635afc21b100404037bc47d31e173c3cc67791"),
		WasmFilename:            "circuit.wasm",
	},
}

// GetCircuitConfiguration returns the circuit configuration associated to the
// provided tag or gets the default one.
func GetCircuitConfiguration(configTag string) ZkCircuitConfig {
	circuitConf := CircuitsConfigurations[DefaultCircuitConfigurationTag]
	if conf, ok := CircuitsConfigurations[configTag]; ok {
		circuitConf = conf
	}
	return circuitConf
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
