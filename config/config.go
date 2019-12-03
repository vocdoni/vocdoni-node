package config

// GWCfg stores global configs for gateway
type GWCfg struct {
	W3      W3Cfg
	Vochain VochainCfg
	Ipfs    IPFSCfg
	Dvote   struct {
		Host string
		Port int
	}
	Api struct {
		Route string
		File  struct {
			Enabled bool
		}
		Census struct {
			Enabled bool
		}
		Vote struct {
			Enabled bool
		}
	}
	Client EthereumClient
	Ssl    struct {
		Domain  string
		DirCert string
	}
	ListenPort int
	ListenHost string
	LogLevel   string
	LogOutput  string
	DataDir    string
	W3external string
}

type IPFSCfg struct {
	ConfigPath string
	Daemon     string
	NoInit     bool
	SyncKey    string
	SyncPeers  []string
}

type EthereumClient struct {
	AllowPrivate bool
	AllowedAddrs string
	SigningKey   string
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

type PssMetaCfg struct {
	LogLevel   string
	Datadir    string
	ListenHost string
	ListenPort int16
}

// W3Cfg stores global configs for web3
type W3Cfg struct {
	Enabled   bool
	ChainType string
	LightMode bool
	WsHost    string
	WsPort    int
	HttpHost  string
	HttpPort  int
	NodePort  int
	LogLevel  string
	Route     string
	DataDir   string
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

// RelayCfg stores global configs for relay
type RelayCfg struct {
	LogLevel          string
	TransportIDString string
	StorageIDString   string
	IpfsConfigPath    string
}

type GenCfg struct {
	LogLevel    string
	Target      string
	Connections int
	Interval    int
}

// TO MODIFY
// Basic app config for now, TBD: Create Vochain config file from gateway flags
type VochainCfg struct {
	LogLevel         string
	ConfigFilePath   string
	TendermintConfig string
	RpcListen        string
	P2pListen        string
	PublicAddr       string
	DataDir          string
	Genesis          string
	CreateGenesis    bool
	Peers            []string
	Seeds            []string
	SeedMode         bool
	KeyFile          string
	MinerKeyFile     string
	Contract         string
}

type OracleCfg struct {
	EthereumConfig W3Cfg
	EthereumClient EthereumClient
	VochainConfig  VochainCfg
	// The expected VochainClient field does not exists because no needs config
	Ipfs          IPFSCfg
	LogLevel      string
	LogOutput     string
	DataDir       string
	W3external    string
	SubscribeOnly bool
}
