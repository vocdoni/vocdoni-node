package config

//GWCfg stores global configs for gateway
type GWCfg struct {
	W3      W3Cfg
	Cluster ClusterCfg
	Ipfs    struct {
		ConfigPath string
		Daemon     string
		NoInit     bool
	}
	Dvote struct {
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
	Client struct {
		AllowPrivate bool
		AllowedAddrs string
		SigningKey   string
	}
	Ssl struct {
		Domain  string
		DirCert string
	}
	ListenPort int
	ListenHost string
	LogLevel   string
	DataDir    string
	W3external string
}

//PssCfg stores global configs for Pss chat
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

//W3Cfg stores global configs for web3
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

//CensusCfg stores global configs for censushttp
type CensusCfg struct {
	LogLevel  string
	Port      int
	SignKey   string
	DataDir   string
	SslDomain string
	RootKey   string
}

//RelayCfg stores global configs for relay
type RelayCfg struct {
	LogLevel          string
	TransportIDString string
	StorageIDString   string
	IpfsConfigPath    string
	Cluster           ClusterCfg
}

type GenCfg struct {
	LogLevel    string
	Target      string
	Connections int
	Interval    int
}
