package config

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
	// EthEventConfig ethereum even subscription config options
	EthEventConfig *EthEventCfg
	// API api config options
	API *API
	// Metrics config options
	Metrics *MetricsCfg
	// Dev indicates we use the gateway development mode
	Dev bool
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
}

// ValidMode checks if the configured mode is valid
func (c *DvoteCfg) ValidMode() bool {
	switch c.Mode {
	case "gateway":
		break
	case "oracle":
		break
	case "miner":
		break
	case "web3":
		break
	default:
		return false
	}
	return true
}

// NewGatewayConfig initializes the fields in the gateway config stuct
func NewConfig() *DvoteCfg {
	return &DvoteCfg{
		W3Config:       new(W3Cfg),
		VochainConfig:  new(VochainCfg),
		Ipfs:           new(IPFSCfg),
		EthConfig:      new(EthCfg),
		EthEventConfig: new(EthEventCfg),
		API:            new(API),
		Metrics:        new(MetricsCfg),
	}
}

// API includes information required by the api, which modules are enabled and the route
type API struct {
	Route   string
	File    bool
	Census  bool
	Vote    bool
	Results bool
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
	Daemon    string
	NoInit    bool
	SyncKey   string
	SyncPeers []string
}

// EthCfg stores global configs for ethereum bockchain
type EthCfg struct {
	// ChainType chain to connect with
	ChainType string
	// LightMode use ethereum node in light mode
	LightMode bool
	// NodePort port annouced for p2p connections
	NodePort int
	// LogLevel logging level
	LogLevel string
	// DataDir path indicating where the ethereum related data will be stored
	DataDir string
	// SigningKey key used to sign transactions
	SigningKey string
	// BootNodes list for bootstraping the Ethereum network
	BootNodes []string
	// TrustedPeers list of p2p Ethereum peers to trust and connect (if possible)
	TrustedPeers []string
	// ProcessDomain ethereum contract to use as source of truth for some operations
	ProcessDomain string
	// NoWaitSync do not wait for Ethereum to synchronize
	NoWaitSync bool
}

// W3Cfg stores global configs for web3
type W3Cfg struct {
	// Enabled if true w3 will be initialized
	Enabled bool
	// RPCHost node websocket host endpoint
	RPCHost string
	// RPCPort node websocket port endpoint
	RPCPort int
	// Route web3 route endpoint
	Route string
	// W3External URL of an external ethereum node to connect with
	W3External string
}

type EthEventCfg struct {
	// CensusSync if true census sync will be enabled
	CensusSync bool
	// SubscribeOnly if true only new received events will be processed, otherwise all events of the current chain will be processed
	SubscribeOnly bool
}

// VochainCfg includes all possible config params needed by the Vochain
type VochainCfg struct {
	// Dev indicates we use the Vochain development mode
	// currently only changes the seed nodes to connect with
	Dev bool
	// LogLevel logging level
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
	// Genesis path where the genesis file is stored
	Genesis string
	// CreateGenesis if True a new genesis file is created
	CreateGenesis bool
	// Peers peers with which the node tries to connect
	Peers []string
	// Seeds seeds with which the node tries to connect
	Seeds []string
	// SeedMode if True the node will act as a seed node
	SeedMode bool
	// MinerKey contains the EDDSA private key for signing tendermint blocks
	MinerKey string
	// NodeKey contains the EDDSA public key that identifies the node in the P2P network
	NodeKey string
	// NoWaitSync if enabled the Vochain synchronization won't be blocking
	NoWaitSync bool
	// SaveConfig overwrites the config file with the CLI provided flags
	SaveConfig bool
	// MempoolSize is the size of the mempool
	MempoolSize int
	// KeyKeeperIndex is the index used by the key keeper (usualy and oracle)
	KeyKeeperIndex int8
	// ImportPreviousCensus if true the census downloader will try to download all census (not only the new ones)
	ImportPreviousCensus bool
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
