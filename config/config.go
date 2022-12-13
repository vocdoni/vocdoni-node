package config

import (
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/types"
)

// DvoteCfg stores global configs for dvote
type Config struct {
	// Vochain config options
	Vochain *VochainCfg
	// Ipfs ipfs config options
	Ipfs *IPFSCfg
	// Metrics config options
	Metrics *MetricsCfg
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// LogErrorFile for logging warning, error and fatal messages
	LogErrorFile string
	// DataDir path where the gateway files will be stored
	DataDir string
	// SaveConfig overwrites the config file with the CLI provided flags
	SaveConfig bool
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
	// EnableRPC enables the HTTP RPC service
	EnableRPC bool
}

// ValidMode checks if the configured mode is valid
func (c *Config) ValidMode() bool {
	switch c.Mode {
	case types.ModeGateway:
		break
	case types.ModeOracle:
		break
	case types.ModeMiner:
		break
	case types.ModeSeed:
		break
	default:
		return false
	}
	return true
}

// ValidDBType checks if the configured dbType is valid
func (c *VochainCfg) ValidDBType() bool {
	switch c.DBType {
	case db.TypePebble:
		break
	case db.TypeBadger:
		break
	default:
		return false
	}
	return true
}

// NewConfig initializes the fields in the config stuct.
func NewConfig() *Config {
	return &Config{
		Vochain: new(VochainCfg),
		Ipfs:    new(IPFSCfg),
		Metrics: new(MetricsCfg),
	}
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
	// Chain is the network name to connect with
	Chain string
	// Dev indicates we use the Vochain development mode (low security is accepted)
	Dev bool
	// LogLevel logging level for tendermint
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// RPCListen address for the RPC server tendermint listens on
	RPCListen string
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
	// MinerKey contains the EDDSA private key for signing tendermint blocks
	MinerKey string
	// NodeKey contains the EDDSA public key that identifies the node in the P2P network
	NodeKey string
	// PrivValidatorAddr if defined, Tendermint node will open a port and wait for a private validator connection
	// (example value: tcp://0.0.0.0:26658)
	PrivValidatorListenAddr string
	// NoWaitSync if enabled the Vochain synchronization won't be blocking
	NoWaitSync bool
	// MempoolSize is the size of the mempool
	MempoolSize int
	// KeyKeeperIndex is the index used by the key keeper (usually and oracle)
	KeyKeeperIndex int8
	// ImportPreviousCensus if true the census downloader will try to download
	// all census (not only the new ones)
	ImportPreviousCensus bool
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
	OffChainDataDownloader bool
}

// IndexerCfg handles the configuration options of the indexer
type IndexerCfg struct {
	// Enables Indexer
	Enabled bool
	// Disables live results computation on indexer
	IgnoreLiveResults bool
}

// OracleCfg includes all possible config params needed by the Oracle
type OracleCfg struct {
	// Dev indicates we use the development mode
	Dev bool
	// DataDir directory where data and config files are stored
	DataDir string
	// VochainConfig vochain node config parameters
	VochainConfig *VochainCfg
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
}

// NewOracleCfg initializes the Oracle config
func NewOracleCfg() *OracleCfg {
	return &OracleCfg{
		VochainConfig: new(VochainCfg),
	}
}

// MetricsCfg initializes the metrics config
type MetricsCfg struct {
	Enabled         bool
	RefreshInterval int
}

// TODO(mvdan): replace with a special error type

// Error helps to handle better config errors on startup
type Error struct {
	// Critical indicates if the error encountered is critical and the app must be stopped
	Critical bool
	// Message error message
	Message string
}
