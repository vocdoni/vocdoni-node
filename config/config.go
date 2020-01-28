package config

// GWCfg stores global configs for gateway
type GWCfg struct {
	// W3 ethereum config options
	W3 *W3Cfg
	// Vochain vochain config options
	Vochain *VochainCfg
	// Ipfs ipfs config options
	Ipfs *IPFSCfg
	// Client ethereum client config options
	Ethereum *EthCfg
	// API api config options
	API *API
	// Ssl tls related config options
	Ssl struct {
		Domain  string
		DirCert string
	}
	// ListenPort port where the gateway will listen on
	ListenPort int
	// ListenHost host where the gateway will listen on
	ListenHost string
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// DataDir path where the gateway files will be stored
	DataDir string
	// W3External ethereum external endpoint to connect with
	W3External string
	// CensusSync if true census sync will be enabled
	CensusSync bool
	// Contract ethereum contract to use as source of truth for some operations
	Contract string
	// ConfigFilePath path indicating where is the gateway config file or where to store if not present
	ConfigFilePath string
}

// NewGatewayConfig initializes the fields in the gateway config stuct
func NewGatewayConfig() *GWCfg {
	return &GWCfg{
		W3:       new(W3Cfg),
		Vochain:  new(VochainCfg),
		Ipfs:     new(IPFSCfg),
		Ethereum: new(EthCfg),
		API:      new(API),
		Ssl: struct {
			Domain  string
			DirCert string
		}{},
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

// PssCfg stores global configs for Pss chat
type PssCfg struct {
	Encryption string
	Key        string
	Topic      string
	Address    string
	Nick       string
	Datadir    string
	Light      bool
	PingMode   bool
	LogLevel   string
	Bootnode   string
}

// PssMetaCfg pss meta configuration
type PssMetaCfg struct {
	LogLevel   string
	Datadir    string
	ListenHost string
	ListenPort int16
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
}

// W3Cfg stores global configs for web3
type W3Cfg struct {
	// Enabled if true w3 will be initialized
	Enabled bool
	// WsHost node websocket host endpoint
	WsHost string
	// WsPort node websocket port endpoint
	WsPort int
	// HTTPHost node http host endpoint
	HTTPHost string
	// HTTPPort node http port endpoint
	HTTPPort int
	// Route web3 route endpoint
	Route string
	// W3External URL of an external ethereum node to connect with
	W3External string
	// HTTPAPI if true and local node http api will be available
	HTTPAPI bool
	// WSAPI if true and local node ws api will be available
	WSAPI bool
}

// CensusCfg stores global configs for censushttp
type CensusCfg struct {
	LogLevel  string
	Port      int
	SignKey   string
	DataDir   string
	SslDomain string
	RootKey   string
}

type GenCfg struct {
	LogLevel    string
	Target      string
	Connections int
	Interval    int
}

// VochainCfg includes all possible config params needed by the Vochain
type VochainCfg struct {
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
	// KeyFile the node key file, needed to identify the node in a P2P network
	KeyFile string
	// MinerKeyFile the node private validator key file, needed to sign consensus messages
	MinerKeyFile string
}

// OracleCfg includes all possible config params needed by the Oracle
type OracleCfg struct {
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
	// Contract address of the ethereum voting smart contract
	Contract string
}

// NewOracleCfg initializes the Oracle config
func NewOracleCfg() *OracleCfg {
	return &OracleCfg{
		EthConfig:     new(EthCfg),
		W3Config:      new(W3Cfg),
		VochainConfig: new(VochainCfg),
	}
}

// Error helps to handle better config errors on startup
type Error struct {
	// Critical indicates if the error encountered is critical and the app must be stopped
	Critical bool
	// Message error message
	Message string
}
