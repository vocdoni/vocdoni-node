package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/ipfssync"
	"go.vocdoni.io/dvote/log"
)

// makes deprecated flags work the same as the new flags, but prints a warning
func deprecatedFlagsFunc(f *flag.FlagSet, name string) flag.NormalizedName {
	oldName := name
	switch name {
	case "helloTime":
		name = "helloInterval"
	case "updateTime":
		name = "updateInterval"
	}
	if oldName != name {
		fmt.Printf("Flag --%s has been deprecated, please use --%s instead\n", oldName, name)
	}
	return flag.NormalizedName(name)
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	userDir := home + "/.ipfs"
	logLevel := flag.StringSlice("logLevel", []string{"info"}, "log level")
	dataDir := flag.String("dataDir", userDir, "directory for storing data")
	key := flag.String("key", "vocdoni", "secret shared group key for the sync cluster")
	nodeKey := flag.String("nodeKey", "", "custom private hexadecimal 256 bit key for p2p identity")
	port := flag.Int16("port", 4171, "port for the sync network")
	helloInterval := flag.Int("helloInterval", 40, "period in seconds for sending hello messages")
	updateInterval := flag.Int("updateInterval", 20, "period in seconds for sending update messages")
	peers := flag.StringSlice("peers", []string{},
		"custom list of peers to connect to (multiaddresses separated by commas)")
	private := flag.Bool("private", false,
		"if enabled a private libp2p network will be created (using the secret key at transport layer)")
	bootnodes := flag.StringSlice("bootnodes", []string{},
		"list of bootnodes (multiaddresses separated by commas)")
	bootnode := flag.Bool("bootnode", false,
		"act as a bootstrap node (will not try to connect with other bootnodes)")

	flag.CommandLine.SortFlags = false
	flag.CommandLine.SetNormalizeFunc(deprecatedFlagsFunc)
	flag.Parse()

	log.Init(strings.Join(*logLevel, ","), "stdout")
	ipfsStore := data.IPFSNewConfig(*dataDir)
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		log.Fatal(err)
	}

	sk := ethereum.NewSignKeys()
	var privKey string

	if len(*nodeKey) > 0 {
		if err := sk.AddHexKey(*nodeKey); err != nil {
			log.Fatal(err)
		}
		_, privKey = sk.HexString()
	} else {
		pk := make([]byte, 64)
		kfile, err := os.OpenFile(*dataDir+"/.ipfsSync.key", os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			log.Fatal(err)
		}

		if n, err := kfile.Read(pk); err != nil || n == 0 {
			log.Info("generating new node private key")
			if err := sk.Generate(); err != nil {
				log.Fatal(err)
			}
			_, privKey = sk.HexString()
			if _, err := kfile.WriteString(privKey); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Info("loaded saved node private key")
			if err := sk.AddHexKey(string(pk)); err != nil {
				log.Fatal(err)
			}
			_, privKey = sk.HexString()
		}
		if err := kfile.Close(); err != nil {
			log.Fatal(err)
		}
	}

	p2pType := "libp2p"
	if *private {
		p2pType = "privlibp2p"
	}

	is := ipfssync.NewIPFSsync(*dataDir, *key, privKey, p2pType, storage)
	is.HelloInterval = time.Second * time.Duration(*helloInterval)
	is.UpdateInterval = time.Second * time.Duration(*updateInterval)
	is.Port = *port
	if *bootnode {
		is.Bootnodes = []string{""}
	} else {
		is.Bootnodes = *bootnodes
	}
	is.Start()
	for _, peer := range *peers {
		time.Sleep(2 * time.Second)
		log.Infof("connecting to peer %s", peer)
		if err := is.Transport.AddPeer(peer); err != nil {
			log.Warnf("cannot connect to custom peer: (%s)", err)
		}
	}
	// Block forever.
	select {}
}
