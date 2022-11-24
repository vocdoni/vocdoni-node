package config

import (
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/types"
)

// DvoteCfg stores global configs for dvote
type DvoteCfg struct {
	// W3Config ethereum config options
	W3Config *W3Cfg
	// VochainConfig vochain config options
	VochainConfig *VochainCfg
	// Ipfs ipfs config options
	Ipfs *IPFSCfg
	// EthConfig ethereum client config options
	EthConfig *EthCfg
	// API api config options
	API *API
	// Metrics config options
	Metrics *MetricsCfg
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// ErrorLogFile for logging warning, error and fatal messages
	LogErrorFile string
	// DataDir path where the gateway files will be stored
	DataDir string
	// SaveConfig overwrites the config file with the CLI provided flags
	SaveConfig bool
	// Mode describes the operation mode of program
	Mode string
	// Dev enables the development mode (less security)
	Dev bool
	// PprofPort is the port where pprof http endpoint listen if dev enabled
	PprofPort int
}

// ValidMode checks if the configured mode is valid
func (c *DvoteCfg) ValidMode() bool {
	switch c.Mode {
	case types.ModeGateway:
		break
	case types.ModeOracle:
		break
	case types.ModeMiner:
		break
	case types.ModeEthAPIoracle:
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

// NewGatewayConfig initializes the fields in the gateway config stuct
func NewConfig() *DvoteCfg {
	return &DvoteCfg{
		W3Config:      new(W3Cfg),
		VochainConfig: new(VochainCfg),
		Ipfs:          new(IPFSCfg),
		EthConfig:     new(EthCfg),
		API:           new(API),
		Metrics:       new(MetricsCfg),
	}
}

// API includes information required by the api, which modules are enabled and the route
type API struct {
	Route   string
	File    bool
	Census  bool
	Vote    bool
	Results bool
	Indexer bool
	URL     bool
	// AllowPrivate allow to use private methods
	AllowPrivate bool
	// AllowedAddrs allowed addresses to interact with
	AllowedAddrs string
	// ListenPort port where the API server will listen on
	ListenPort int
	// ListenHost host where the API server will listen on
	ListenHost string
	// Ssl tls related config options
	Ssl struct {
		Domain  string
		DirCert string
	}
}

// IPFSCfg includes all possible config params needed by IPFS
type IPFSCfg struct {
	// ConfigPath root path used by IPFS running node
	ConfigPath string
	// Daemon
	Daemon       string
	NoInit       bool
	ConnectKey   string
	ConnectPeers []string
}

// EthCfg stores global configs for ethereum bockchain
type EthCfg struct {
	// SigningKey key used to sign transactions
	SigningKey string
}

// W3Cfg stores global configs for web3
type W3Cfg struct {
	// ChainType chain to connect with
	ChainType string
	// W3External URLs of an external ethereum nodes to connect with
	W3External []string
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
	// RPCListen address for the RPC server tp listen on
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
	// CreateGenesis if True a new genesis file is created
	CreateGenesis bool
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
	// SaveConfig overwrites the config file with the CLI provided flags
	SaveConfig bool
	// MempoolSize is the size of the mempool
	MempoolSize int
	// KeyKeeperIndex is the index used by the key keeper (usually and oracle)
	KeyKeeperIndex int8
	// ImportPreviousCensus if true the census downloader will try to download
	// all census (not only the new ones)
	ImportPreviousCensus bool
	// Enable Prometheus metrics from tendermint
	TendermintMetrics bool
	// EthereumWhiteListAddrs is a map of ethereum addresses able to create oracle txs
	// If the ethereum address that modified the source of truth
	// is not on the EthereumWhiteListAddrs the oracle will ignore the event triggered
	// by that transaction
	// If the map is empty, any address is accepted
	EthereumWhiteListAddrs []string
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
	// EthereumConfig ethereum node config parameters
	EthConfig *EthCfg
	// W3Config Web3 config parameters
	W3Config *W3Cfg
	// VochainConfig vochain node config parameters
	VochainConfig *VochainCfg
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// SubscribeOnly if true only new received events will be processed, otherwise all events of the current chain will be processed
	SubscribeOnly bool
	// EthProcessDomain ethereum contract to use as source of truth for some operations
	EthProcessDomain string
	// SaveConfig overwrites the config file with the CLI provided flags
	SaveConfig bool
}

// NewOracleCfg initializes the Oracle config
func NewOracleCfg() *OracleCfg {
	return &OracleCfg{
		EthConfig:     new(EthCfg),
		W3Config:      new(W3Cfg),
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
