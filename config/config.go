package config

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

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
		Host  string
		Port  int
		Route string
	}
	Api struct {
		DvoteApi bool
		Web3Api  bool
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
}

//W3Cfg stores global configs for web3
type W3Cfg struct {
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
	LogLevel   string
	Namespaces []string
	Port       int
	SignKey    string
	DataDir    string
	SslDomain  string
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

type ClusterCfg struct {
	Stats           bool
	Tracing         bool
	Consensus       string
	Leave           bool
	Bootstraps      []string
	PinTracker      string
	Alloc           string
	Secret          string
	PeerID          peer.ID
	Private         crypto.PrivKey
	ClusterLogLevel string
}

type ClusterTestCfg struct {
	LogLevel    string
	Targets     []string
	Interval    int
	PkgSize 	int
}