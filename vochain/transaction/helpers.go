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

// LoadZkVKs loads the Zero Knowledge Verification Keys for the given
// ChainID into the BaseApplication, downloading them if necessary, and
// verifying their cryptographic hahes.
// func LoadZkVerificationKeys(dataDir, chainID string) ([]*snarkTypes.Vk, error) {
// 	zkVKs := []*snarkTypes.Vk{}
// 	var circuits []zkartifacts.CircuitConfig
// 	if genesis, ok := vocdoniGenesis.Genesis[chainID]; ok {
// 		circuits = genesis.CircuitsConfig
// 	} else {
// 		log.Info("using dev genesis zkSnarks circuits")
// 		circuits = vocdoniGenesis.Genesis["dev"].CircuitsConfig
// 	}
// 	for i, cc := range circuits {
// 		log.Infof("downloading zk-circuits-artifacts index: %d", i)

// 		// download VKs from CircuitsConfig
// 		ctx, cancel := context.WithTimeout(context.Background(), downloadZkVKsTimeout)
// 		defer cancel()
// 		cc.LocalDir = filepath.Join(dataDir, cc.LocalDir)
// 		if err := zkartifacts.DownloadVKFile(ctx, cc); err != nil {
// 			return nil, err
// 		}

// 		// parse VK and store it into vnode.ZkVKs
// 		log.Infof("parse VK from file into memory. CircuitArtifact index: %d", i)
// 		vk, err := zk.LoadVkFromFile(filepath.Join(cc.LocalDir, cc.CircuitPath, zkartifacts.FilenameVK))
// 		if err != nil {
// 			return nil, err
// 		}
// 		zkVKs = append(zkVKs, vk)
// 	}
// 	return zkVKs, nil
// }

func LoadZkCircuits(dataDir, chainID string) ([]*circuit.ZkCircuit, error) {
	zkCircuits := []*circuit.ZkCircuit{}
	var circuits []circuit.ZkCircuitConfig
	if genesis, ok := vocdoniGenesis.Genesis[chainID]; ok {
		circuits = genesis.CircuitsConfig
	} else {
		log.Info("using dev genesis zkSnarks circuits")
		circuits = vocdoniGenesis.Genesis["dev"].CircuitsConfig
	}

	for i, config := range circuits {
		log.Infof("downloading zk-circuits-artifacts index: %d", i)

		// download VKs from CircuitsConfig
		ctx, cancel := context.WithTimeout(context.Background(), downloadZkVKsTimeout)
		defer cancel()
		config.LocalDir = filepath.Join(dataDir, config.LocalDir)

		zkCircuit, err := circuit.LoadZkCircuit(ctx, config)
		if err != nil {
			return nil, err
		}

		zkCircuits = append(zkCircuits, zkCircuit)
	}
	return zkCircuits, nil
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
