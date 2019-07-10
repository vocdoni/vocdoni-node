package config

//GWCfg stores global configs for gateway
type GWCfg struct {
	W3   W3Cfg
	Ipfs struct {
		Daemon string
		NoInit bool
	}
	Dvote struct {
		Host  string
		Port  int
		Route string
	}
	Api struct {
		FileApi bool
		Web3Api bool
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
	LogLevel string
	DataDir  string
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
}

//W3Cfg stores global configs for web3
type W3Cfg struct {
	ChainType string
	WsHost    string
	WsPort    int
	HttpHost  string
	HttpPort  int
	NodePort  int
	LogLevel  string
	Route     string
}

//CensusCfg stores global configs for censushttp
type CensusCfg struct {
	LogLevel string
}

//RelayCfg stores global configs for relay
type RelayCfg struct {
	LogLevel          string
	TransportIDString string
	StorageIDString   string
}

type GenCfg struct {
	LogLevel    string
	Target      string
	Connections int
	Interval    int
}
