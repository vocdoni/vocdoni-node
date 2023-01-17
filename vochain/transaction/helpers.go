package transaction

import (
	"context"
	"path/filepath"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/log"
	vocdoniGenesis "go.vocdoni.io/dvote/vochain/genesis"
)

const downloadZkVKsTimeout = 1 * time.Minute

func LoadZkCircuits(dataDir, chainID string) (*circuit.ZkCircuit, error) {
	circuitConf := circuit.DefaultCircuitsConfiguration
	if genesis, ok := vocdoniGenesis.Genesis[chainID]; ok {
		circuitConf = circuit.CircuitsConfigurations[genesis.CircuitsConfigTag]
	} else {
		log.Info("using dev genesis zkSnarks circuits")
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
