package config

import (
	"go.vocdoni.io/dvote/types"
)

// Config stores global configurations for vocdoni-node
type Config struct {
	// Vochain config options
	Vochain VochainCfg
	// Ipfs config options
	Ipfs IPFSCfg
	// Metrics config options
	Metrics MetricsCfg
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// LogErrorFile for logging warning, error and fatal messages
	LogErrorFile string
	// DataDir path where the gateway files will be stored
	DataDir string
	// SigningKey key used to sign transactions
	SigningKey string
	// Mode describes the operation mode of program
	Mode string
	// Dev enables the development mode (less security)
	Dev bool
	// PprofPort is the port where pprof http endpoint listen if dev enabled
	PprofPort int
	// ListenPort port where the API server will listen on
	ListenPort int
	// ListenHost host where the API server will listen on
	ListenHost string
	// TLS related config options
	TLS struct {
		Domain  string
		DirCert string
	}
	// EnableAPI enables the HTTP API REST service
	EnableAPI bool
	// AdminAPIToken is the token used to authenticate admin API requests
	AdminToken string
	// EnableFaucet enables the faucet API service for the given amounts
	EnableFaucetWithAmount uint64
}

// ValidMode checks if the configured mode is valid
func (c *Config) ValidMode() bool {
	switch c.Mode {
	case types.ModeGateway:

	case types.ModeMiner:

	case types.ModeSeed:

	case types.ModeCensus:

	default:
		return false
	}
	return true
}

// IPFSCfg includes all possible config params needed by IPFS
type IPFSCfg struct {
	// ConfigPath root path used by IPFS running node
	ConfigPath string
	// ConnectKey is the ipfsConnect secret key for finding cluster peers
	ConnectKey string
	// ConnectPeers is the list of ipfsConnect peers
	ConnectPeers []string
}

// VochainCfg includes all possible config params needed by the Vochain
type VochainCfg struct {
	// Network is the network name to connect with
	Network string
	// Dev indicates we use the Vochain development mode (low security is accepted)
	Dev bool
	// LogLevel logging level for tendermint
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// P2PListen address to listen for incoming P2P connections
	P2PListen string
	// PublicAddr IP address to expose, guessed by the program (your public IP address) if not set
	PublicAddr string
	// DataDir directory where the Vochain related data (DB's and priv_validator_state.json) is stored
	DataDir string
	// DBType is the type of key-value db to be used
	DBType string
	// Genesis path where the genesis file is stored
	Genesis string
	// Peers peers with which the node tries to connect
	Peers []string
	// Seeds seeds with which the node tries to connect
	Seeds []string
	// MinerKey contains the secp256k1 private key for signing tendermint blocks
	MinerKey string
	// NodeKey contains the ed25519 public key that identifies the node in the P2P network
	NodeKey string
	// PrivValidatorAddr if defined, Tendermint node will open a port and wait for a private validator connection
	// (example value: tcp://0.0.0.0:26658)
	PrivValidatorListenAddr string
	// NoWaitSync if enabled the Vochain synchronization won't be blocking
	NoWaitSync bool
	// AutoWipeDataDir enables wiping out the whole datadir when hardcoded genesis changes
	AutoWipeDataDir bool
	// AutoWipeCometBFT enables wiping out the cometbft datadir when hardcoded genesis changes
	AutoWipeCometBFT bool
	// MempoolSize is the size of the mempool
	MempoolSize int
	// SkipPreviousOffchainData if enabled, the node will skip downloading the previous off-chain data to the current block
	SkipPreviousOffchainData bool
	// Enable Prometheus metrics from tendermint
	TendermintMetrics bool
	// Target block time in seconds (only for miners)
	MinerTargetBlockTimeSeconds int
	// Enables the process archiver component
	ProcessArchive bool
	// Base64 IPFS private key for using with the process archive
	ProcessArchiveKey string
	// Data directory for storing the process archive
	ProcessArchiveDataDir string
	// Indexer holds the configuration regarding the indexer component
	Indexer IndexerCfg
	// IsSeedNode specifies if the node is configured to act as a seed node
	IsSeedNode bool
	// OffChainDataDownload specifies if the node is configured to download off-chain data
	OffChainDataDownload bool
	// SnapshotInterval enables creating a state snapshot every N blocks (0 to disable)
	SnapshotInterval int
	// StateSyncEnabled allows cometBFT during startup, to ask peers for available snapshots
	// and use them to bootstrap the state
	StateSyncEnabled bool
	// StateSyncRPCServers is the list of RPC servers to bootstrap the StateSync (optional, defaults to using seeds)
	StateSyncRPCServers []string
	// StateSyncTrustHeight and TrustHash must both be provided to force the trusting of a
	// particular header.
	StateSyncTrustHeight int64
	// StateSyncTrustHeight and TrustHash must both be provided to force the trusting of a
	// particular header.
	StateSyncTrustHash string
	// StateSyncChunkSize defines the size of the chunks when splitting a Snapshot for sending via StateSync
	StateSyncChunkSize int64
	// StateSyncFetchParamsFromRPC allows statesync to fetch TrustHash and TrustHeight from the first RPCServer
	StateSyncFetchParamsFromRPC bool
}

// IndexerCfg handles the configuration options of the indexer
type IndexerCfg struct {
	// Enables Indexer
	Enabled bool
	// Disables live results computation on indexer
	IgnoreLiveResults bool
	// ArchiveURL is the URL where the archive is retrieved from (usually IPNS)
	ArchiveURL string
}

// MetricsCfg initializes the metrics config
type MetricsCfg struct {
	Enabled         bool
	RefreshInterval int
}
