package apiclient

import (
	"encoding/hex"
	"log"
	"strings"

	"go.vocdoni.io/dvote/crypto/zk/circuit"
)

// Constant that stores the default index of the circuit configuration into the
// vochain genesis configuration. It will be deprecated when the vochain has only
// one circuit.
const ZKCircuitIndex int32 = 1

// CircuitsConfiguration stores the relation between the different vochain nets
// and the associated circuit configuration. Any circuit configuration must have
// the remote and local location of the circuits artifacts and their metadata
// such as artifacts hash or the number of parameters.
var CircuitsConfigurations = map[string]circuit.ZkCircuitConfig{
	"dev": { // index: 1, size: 65k
		URI: "https://raw.githubusercontent.com/vocdoni/" +
			"zk-franchise-proof-circuit/feature/merging_repos_and_new_tests",
		CircuitPath:         "artifacts/zkCensus/dev/16",
		Parameters:          []int64{65536}, // 2^16
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
