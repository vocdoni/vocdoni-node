// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain/genesis"

	cometconfig "github.com/cometbft/cometbft/config"
	cometp2p "github.com/cometbft/cometbft/p2p"
	cometproxy "github.com/cometbft/cometbft/proxy"

	cometnode "github.com/cometbft/cometbft/node"
	"go.vocdoni.io/dvote/log"
)

// NewVochain starts a node with an ABCI application
func NewVochain(vochaincfg *config.VochainCfg) *BaseApplication {
	migrateLegacyDirs(vochaincfg.DataDir)
	// creating new vochain app
	app, err := NewBaseApplication(vochaincfg)
	if err != nil {
		log.Fatalf("cannot initialize vochain application: %s", err)
	}
	log.Info("creating tendermint node and application")
	err = app.SetNode(vochaincfg)
	if err != nil {
		log.Fatal(err)
	}
	// Set the vote cache at least as big as the mempool size
	if app.State.CacheSize() < vochaincfg.MempoolSize {
		app.State.SetCacheSize(vochaincfg.MempoolSize)
	}
	return app
}

// newTendermint creates a new tendermint node attached to the given ABCI app
func newTendermint(app *BaseApplication, localConfig *config.VochainCfg) (*cometnode.Node, error) {
	var err error

	tconfig := cometconfig.DefaultConfig()

	tconfig.SetRoot(filepath.Join(localConfig.DataDir, config.DefaultCometBFTPath))
	if err := os.MkdirAll(filepath.Join(tconfig.RootDir, cometconfig.DefaultConfigDir), 0o750); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tconfig.RootDir, cometconfig.DefaultDataDir), 0o750); err != nil {
		log.Fatal(err)
	}

	tconfig.LogLevel = localConfig.LogLevel
	if tconfig.LogLevel == "none" {
		tconfig.LogLevel = "disabled"
	}
	tconfig.P2P.ExternalAddress = localConfig.PublicAddr
	if localConfig.Dev {
		tconfig.P2P.AllowDuplicateIP = true
		tconfig.P2P.AddrBookStrict = false
	}
	tconfig.P2P.Seeds = strings.Trim(strings.Join(localConfig.Seeds, ","), "[]\"")
	if len(tconfig.P2P.Seeds) < 8 && !localConfig.IsSeedNode {
		if seeds, ok := config.DefaultSeedNodes[localConfig.Network]; ok {
			tconfig.P2P.Seeds = strings.Join(seeds, ",")
		}
	}
	if len(tconfig.P2P.Seeds) > 0 {
		log.Infof("seed nodes: %s", tconfig.P2P.Seeds)
	}

	if len(localConfig.Peers) > 0 {
		tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers, ","), "[]\"")
	}
	if len(tconfig.P2P.PersistentPeers) > 0 {
		log.Infof("persistent peers: %s", tconfig.P2P.PersistentPeers)
	}

	// consensus config
	blockTime := config.DefaultMinerTargetBlockTimeSeconds
	if localConfig.MinerTargetBlockTimeSeconds > 0 {
		blockTime = localConfig.MinerTargetBlockTimeSeconds
	}
	tconfig.Consensus.TimeoutProposeDelta = time.Millisecond * 200
	tconfig.Consensus.TimeoutPropose = time.Second * time.Duration(float32(blockTime)*0.6)
	tconfig.Consensus.TimeoutPrevoteDelta = time.Millisecond * 200
	tconfig.Consensus.TimeoutPrevote = time.Second * 1
	tconfig.Consensus.TimeoutPrecommitDelta = time.Millisecond * 200
	tconfig.Consensus.TimeoutPrecommit = time.Second * 1
	tconfig.Consensus.TimeoutCommit = time.Second * time.Duration(blockTime)

	// if seed node
	if localConfig.IsSeedNode {
		tconfig.P2P.SeedMode = true
	}

	log.Infow("consensus time target",
		"precommit", tconfig.Consensus.TimeoutPrecommit.Seconds(),
		"propose", tconfig.Consensus.TimeoutPropose.Seconds(),
		"prevote", tconfig.Consensus.TimeoutPrevote.Seconds(),
		"commit", tconfig.Consensus.TimeoutCommit.Seconds(),
		"block", blockTime)

	// disable transaction indexer (we don't use it)
	tconfig.TxIndex = &cometconfig.TxIndexConfig{Indexer: "null"}
	// mempool config
	tconfig.Mempool.Size = localConfig.MempoolSize
	tconfig.Mempool.Recheck = true
	tconfig.Mempool.KeepInvalidTxsInCache = true
	tconfig.Mempool.MaxTxBytes = 1024 * 100 // 100 KiB
	tconfig.Mempool.MaxTxsBytes = int64(tconfig.Mempool.Size * tconfig.Mempool.MaxTxBytes)
	tconfig.Mempool.CacheSize = 100000
	tconfig.Mempool.Broadcast = true

	tconfig.StateSync.Enable = localConfig.StateSyncEnabled
	// if State is already init'ed (height > 0) then skip StateSync entirely
	if app.State != nil {
		if height, err := app.State.LastHeight(); err == nil && height > 0 {
			tconfig.StateSync.Enable = false
		}
	}

	if tconfig.StateSync.Enable {
		tconfig.StateSync.RPCServers = func() []string {
			// prefer the most the specific flag first
			if len(localConfig.StateSyncRPCServers) > 0 {
				return localConfig.StateSyncRPCServers
			}

			// else, we resort to seeds (replacing the port)
			replacePorts := func(slice []string) []string {
				for i, v := range slice {
					slice[i] = strings.ReplaceAll(v, ":26656", ":26657")
				}
				return slice
			}

			// first fallback to Seeds
			if len(localConfig.Seeds) > 0 {
				return replacePorts(localConfig.Seeds)
			}

			// if also no Seeds specified, fallback to genesis
			if seeds, ok := config.DefaultSeedNodes[localConfig.Network]; ok {
				return replacePorts(seeds)
			}

			return nil
		}()

		// after parsing flag and fallbacks, if we still have only 1 server specified,
		// duplicate it as a quick workaround since cometbft requires passing 2 (primary and witness)
		if len(tconfig.StateSync.RPCServers) == 1 {
			tconfig.StateSync.RPCServers = append(tconfig.StateSync.RPCServers, tconfig.StateSync.RPCServers...)
		}

		log.Infof("state sync rpc servers: %s", tconfig.StateSync.RPCServers)

		tconfig.StateSync.TrustHeight = localConfig.StateSyncTrustHeight
		tconfig.StateSync.TrustHash = localConfig.StateSyncTrustHash

		// If StateSync is enabled but parameters are empty, populate them
		//  fetching params from remote API endpoint
		if localConfig.StateSyncFetchParamsFromRPC &&
			tconfig.StateSync.TrustHeight == 0 && tconfig.StateSync.TrustHash == "" {
			tconfig.StateSync.TrustHeight, tconfig.StateSync.TrustHash = func() (int64, string) {
				cli, err := newCometRPCClient(tconfig.StateSync.RPCServers[0])
				if err != nil {
					log.Warnf("cannot connect to remote RPC server: %v", err)
					return 0, ""
				}
				status, err := cli.Status(context.TODO())
				if err != nil {
					log.Warnf("cannot fetch status from remote RPC server: %v", err)
					return 0, ""
				}
				// try to get the hash exactly at the snapshot height, to avoid a long verification chain
				height := status.SyncInfo.LatestBlockHeight - (status.SyncInfo.LatestBlockHeight % int64(localConfig.SnapshotInterval))
				b, err := cli.Block(context.TODO(), &height)
				if err == nil {
					log.Infow("fetched statesync params from remote RPC",
						"height", height, "hash", b.BlockID.Hash.String())
					return height, b.BlockID.Hash.String()
				}
				// else at least fallback to the latest height and hash
				log.Infow("fetched statesync params from remote RPC",
					"height", status.SyncInfo.LatestBlockHeight, "hash", status.SyncInfo.LatestBlockHash.String())
				return status.SyncInfo.LatestBlockHeight, status.SyncInfo.LatestBlockHash.String()
			}()
		}
	}
	tconfig.RPC.ListenAddress = "tcp://0.0.0.0:26657"

	log.Debugf("mempool config: %+v", tconfig.Mempool)
	// tmdbBackend defaults to goleveldb, but switches to cleveldb if
	// -tags=cleveldb is used. See tmdb_*.go.
	tconfig.DBBackend = string(tmdbBackend)
	tconfig.Genesis = localConfig.Genesis

	if _, err := os.Stat(tconfig.Genesis); os.IsNotExist(err) {
		log.Infof("writing comet genesis to %s", tconfig.Genesis)
		if err := genesis.HardcodedWithOverrides(localConfig.Network,
			localConfig.GenesisChainID,
			localConfig.GenesisInitialHeight,
			localConfig.GenesisAppHash).SaveAs(tconfig.Genesis); err != nil {
			return nil, fmt.Errorf("cannot write genesis file: %w", err)
		}
	}

	// We need to load the genesis already,
	// to fetch chain_id in order to make Replay work, since signatures depend on it.
	// and also to check InitialHeight to decide what to reply when cometbft ask for Info()
	loadedGenesis, err := genesis.LoadFromFile(tconfig.GenesisFile())
	if err != nil {
		return nil, fmt.Errorf("cannot load genesis file: %w", err)
	}
	log.Infow("loaded genesis file", "genesis", tconfig.GenesisFile(), "chainID", loadedGenesis.ChainID)
	app.genesisDoc = loadedGenesis
	app.SetChainID(loadedGenesis.ChainID)

	// read or create local private validator
	pv, err := NewPrivateValidator(
		localConfig.MinerKey,
		tconfig.PrivValidatorKeyFile(),
		tconfig.PrivValidatorStateFile())
	if err != nil {
		return nil, fmt.Errorf("cannot create validator key and state: (%v)", err)
	}
	pv.Save()
	if pv.Key.PrivKey.Bytes() != nil {
		signer := ethereum.SignKeys{}
		if err := signer.AddHexKey(hex.EncodeToString(pv.Key.PrivKey.Bytes())); err != nil {
			return nil, fmt.Errorf("cannot add private validator key: %w", err)
		}
		// NodeAddress is the ethereum address of the given cometBFT node key
		app.NodeAddress, err = ethereum.AddrFromPublicKey(pv.Key.PubKey.Bytes())
		if err != nil {
			return nil, fmt.Errorf("cannot create node address from pubkey: %w", err)
		}
	}

	// nodekey is used for the p2p transport layer
	nodeKey := new(cometp2p.NodeKey)
	if len(localConfig.NodeKey) > 0 {
		nodeKey, err = NewNodeKey(localConfig.NodeKey, tconfig.NodeKeyFile())
		if err != nil {
			return nil, fmt.Errorf("cannot create node key: %w", err)
		}
	} else {
		if nodeKey, err = cometp2p.LoadOrGenNodeKey(tconfig.NodeKeyFile()); err != nil {
			return nil, fmt.Errorf("cannot create or load node key: %w", err)
		}
	}

	if localConfig.TendermintMetrics {
		tconfig.Instrumentation = &cometconfig.InstrumentationConfig{
			Prometheus:           true,
			PrometheusListenAddr: "",
			MaxOpenConnections:   2,
			Namespace:            "comet",
		}
	}

	if err := tconfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("config is invalid: %w", err)
	}

	log.Infow("vochain initialized",
		"db-backend", tconfig.DBBackend,
		"publicKey", hex.EncodeToString(pv.Key.PubKey.Bytes()),
		"accountAddr", app.NodeAddress,
		"validatorAddr", pv.Key.PubKey.Address(),
		"external-address", tconfig.P2P.ExternalAddress,
		"nodeId", nodeKey.ID(),
		"seed", tconfig.P2P.SeedMode)

	// if GenesisEndOfChain cmdline flag was passed, override genesis
	if localConfig.GenesisEndOfChain > 0 {
		g := genesis.HardcodedForChainID(app.ChainID())
		g.EndOfChain = localConfig.GenesisEndOfChain
		genesis.SetHardcodedForChainID(app.ChainID(), g)
	}
	if genesis.HardcodedForChainID(app.ChainID()).EndOfChain > 0 {
		log.Warnf("this node will stop chain %s at height %d",
			app.ChainID(), genesis.HardcodedForChainID(app.ChainID()).EndOfChain)
	}

	// assign the default tendermint methods
	app.SetDefaultMethods()
	node, err := cometnode.NewNode(
		context.Background(),
		tconfig,
		pv,
		nodeKey,
		cometproxy.NewLocalClientCreator(app),
		cometnode.DefaultGenesisDocProviderFunc(tconfig),
		cometconfig.DefaultDBProvider,
		cometnode.DefaultMetricsProvider(tconfig.Instrumentation),
		log.NewCometLogger("comet", tconfig.LogLevel),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}

	return node, nil
}

// migrateLegacyDirs handles the legacy paths used before commit "refactor genesis package"
// cometbft data is now separated from vcstate and snapshots, so we need to migrate current deployments
func migrateLegacyDirs(dataDir string) {
	// first move our dirs out, so they are not mixed with cometbft data
	// * vochain/data/vcstate -> vochain/vcstate
	// * vochain/data/snapshots -> vochain/snapshots
	migrateLegacyPath(StateDataDir, filepath.Join(dataDir, cometconfig.DefaultDataDir), dataDir)
	migrateLegacyPath(SnapshotsDataDir, filepath.Join(dataDir, cometconfig.DefaultDataDir), dataDir)
	// then move cometbft dirs into its own subdir
	// * vochain/config -> vochain/cometbft/config
	// * vochain/data -> vochain/cometbft/data
	migrateLegacyPath(cometconfig.DefaultConfigDir, dataDir, filepath.Join(dataDir, config.DefaultCometBFTPath))
	migrateLegacyPath(cometconfig.DefaultDataDir, dataDir, filepath.Join(dataDir, config.DefaultCometBFTPath))
}

// migrateLegacyPath moves dir from oldRoot into newRoot,
// only if it exists on oldRoot and not on newRoot
func migrateLegacyPath(dir, oldRoot, newRoot string) {
	oldpath := filepath.Join(oldRoot, dir)
	newpath := filepath.Join(newRoot, dir)
	if _, err := os.Stat(oldpath); os.IsNotExist(err) {
		return
	}
	if _, err := os.Stat(newpath); !os.IsNotExist(err) {
		log.Warnf("found legacy dir %s but also new dir %s, leaving untouched, you need to resolve migration manually", oldpath, newpath)
		return
	}
	if err := os.MkdirAll(newRoot, 0o750); err != nil {
		log.Errorw(err, fmt.Sprintf("migrating legacy dirs, couldn't create %s", newRoot))
		return
	}
	if err := os.Rename(oldpath, newpath); err != nil {
		log.Errorw(err, fmt.Sprintf("couldn't migrate legacy dir %s -> %s", oldpath, newpath))
		return
	}
	log.Warnf("migrated legacy dir %s -> %s", oldpath, newpath)
}
