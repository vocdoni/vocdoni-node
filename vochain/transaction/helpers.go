package transaction

import (
	"context"
	"path/filepath"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/log"
)

const downloadZkVKsTimeout = 1 * time.Minute

func GetZkCircuitByConfigTag(dataDir, configTag string) (*circuit.ZkCircuit, error) {
	circuitConf := circuit.CircuitsConfigurations[circuit.DefaultCircuitConfigurationTag]
	if conf, ok := circuit.CircuitsConfigurations[configTag]; ok {
		circuitConf = conf
	} else {
		log.Info("using default zkSnarks circuit")
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadZkVKsTimeout)
	defer cancel()
	circuitConf.LocalDir = filepath.Join(dataDir, circuitConf.LocalDir)

	zkCircuit, err := circuit.LoadZkCircuit(ctx, circuitConf)
	if err != nil {
		return nil, err
	}

	return zkCircuit, nil
}

// verifySignatureAgainstOracles verifies that a signature match with one of the oracles
func verifySignatureAgainstOracles(oracles []ethcommon.Address, message,
	signature []byte) (bool, ethcommon.Address, error) {
	signKeys := ethereum.NewSignKeys()
	for _, oracle := range oracles {
		signKeys.AddAuthKey(oracle)
	}
	return signKeys.VerifySender(message, signature)
}
