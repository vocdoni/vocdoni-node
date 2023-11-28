package circuit

import (
	"encoding/hex"
	"math/big"

	"go.vocdoni.io/dvote/log"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

// Config defines the configuration of the files to be downloaded
type Config struct {
	// Version of the published circuit
	Version string
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
	// PublicSignals indicates the index of each public signal
	PublicSignals map[string]int
	// maxCensusSize contains a precomputed max size of a census for the
	// circuit, which is defined by the expresion:
	//   maxCensusSize = 2^circuitLevels
	maxCensusSize *big.Int
}

// KeySize returns the maximum number of bytes of a leaf key according to the
// number of levels of the current circuit (nBytes = nLevels / 8).
func (conf *Config) KeySize() int {
	return conf.Levels / 8
}

// MaxCensusSize returns the maximum number of keys that circuit merkle tree
// for the census supports. The method checks if it is already precalculated
// or not. If it is not precalculated, it will calculate and initialise it. In
// any case, the value is returned as big.Int.
func (conf *Config) MaxCensusSize() *big.Int {
	if conf.maxCensusSize != nil {
		return conf.maxCensusSize
	}
	if conf.Levels == 0 {
		log.Fatalf("Circuit levels not defined")
	}
	conf.maxCensusSize = new(big.Int).Exp(big.NewInt(2), new(big.Int).SetInt64(int64(conf.Levels)), nil)
	return conf.maxCensusSize
}

// SupportsCensusSize returns if the provided censusSize is supported by the
// current circuit configuration. It ensures that the provided value is lower
// than 2^config.Levels.
func (conf *Config) SupportsCensusSize(maxCensusSize uint64) bool {
	return conf.MaxCensusSize().Cmp(new(big.Int).SetUint64(maxCensusSize)) > 0
}

// DefaultZkCircuitVersion is the circuit version used by default
const DefaultZkCircuitVersion = V1_0_0

// PreVoceremonyForkZkCircuitVersion is the circuit version used before VoceremonyForkBlock
const PreVoceremonyForkZkCircuitVersion = V0_0_1

// Version strings
const (
	V0_0_1 = "v0.0.1"
	V1_0_0 = "v1.0.0"
)

// CircuitsConfigurations stores the relation between the different vochain nets
// and the associated circuit configuration. Any circuit configuration must have
// the remote and local location of the circuits artifacts and their metadata
// such as artifacts hash or the number of parameters.
var CircuitsConfigurations = map[string]*Config{
	V0_0_1: {
		Version:                 V0_0_1,
		URI:                     "https://raw.githubusercontent.com/vocdoni/zk-franchise-proof-circuit/master",
		CircuitPath:             "artifacts/zkCensus/dev/160",
		Levels:                  160, // ZkCircuit number of levels
		ProvingKeyHash:          hexToBytes("0xe359b256e5e3c78acaccf8dab5dc4bea99a2f07b2a05e935b5ca658c714dea4a"),
		ProvingKeyFilename:      "proving_key.zkey",
		VerificationKeyHash:     hexToBytes("0x235e55571812f8e324e73e37e53829db0c4ac8f68469b9b953876127c97b425f"),
		VerificationKeyFilename: "verification_key.json",
		WasmHash:                hexToBytes("0x80a73567f6a4655d4332301efcff4bc5711bb48176d1c71fdb1e48df222ac139"),
		WasmFilename:            "circuit.wasm",
		PublicSignals: map[string]int{
			"electionId[0]": 0,
			"electionId[1]": 1,
			"nullifier":     2,
			"voteHash[0]":   3,
			"voteHash[1]":   4,
			"sikRoot":       5,
			"censusRoot":    6,
			"voteWeight":    7,
		},
	},
	V1_0_0: {
		Version:                 V1_0_0,
		URI:                     "https://raw.githubusercontent.com/vocdoni/zk-voceremony",
		CircuitPath:             "ceremony/vocdoni-zkcensus-ceremony/results",
		Levels:                  160, // ZkCircuit number of levels
		ProvingKeyHash:          hexToBytes("0x94f4062db3e43175ac1136f285551d547a177e37b0616a41900a38ed5ec3d478"),
		ProvingKeyFilename:      "census_proving_key.zkey",
		VerificationKeyHash:     hexToBytes("0x2a47ff7e511926290fedfa406886944eeb0a3df9021ca26333c0c124c89aa7b0"),
		VerificationKeyFilename: "census_verification_key.json",
		WasmHash:                hexToBytes("0xc98133cf4d84ced677549e0d848739f4e80ddf78af678cbc8b95377247a92773"),
		WasmFilename:            "census.wasm",
		// Due to a bug in this circuit definition, voteWeight ended up being a private signal,
		// and the only public weight-related signal is availableWeight (on index 3).
		// but we don't yet support voteWeight < availableWeight anyway, so we take just availableWeight == voteWeight
		PublicSignals: map[string]int{
			"electionId[0]": 0,
			"electionId[1]": 1,
			"nullifier":     2,
			"voteWeight":    3, // see comment above
			"voteHash[0]":   4,
			"voteHash[1]":   5,
			"sikRoot":       6,
			"censusRoot":    7,
		},
	},
}

// GetCircuitConfiguration returns the circuit configuration associated with the
// provided tag or gets the default one.
func GetCircuitConfiguration(version string) *Config {
	// check if the provided config tag exists and return it if it does
	if conf, ok := CircuitsConfigurations[version]; ok {
		return conf
	}
	// if not, return default configuration
	return CircuitsConfigurations[DefaultZkCircuitVersion]
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
