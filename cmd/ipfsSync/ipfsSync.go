package main

import (
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/ipfssync"
	"gitlab.com/vocdoni/go-dvote/log"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	userDir := home + "/.ipfs"
	logLevel := flag.String("logLevel", "info", "log level")
	dataDir := flag.String("dataDir", userDir, "directory for storing data")
	key := flag.String("key", "vocdoni", "secret shared group key for the sync cluster")
	nodeKey := flag.String("nodeKey", "", "custom private hexadeciaml 256 bit key for p2p identity")
	port := flag.Int16("port", 4171, "port for the sync network")
	helloTime := flag.Int("helloTime", 40, "period in seconds for sending hello messages")
	updateTime := flag.Int("updateTime", 20, "period in seconds for sending update messages")
	peers := flag.StringArray("peers", []string{}, "custom list of peers to connect (multiaddress separated by commas)")
	private := flag.Bool("private", false, "if enabled a private libp2p network will be created (using the secret key at transport layer)")
	bootnodes := flag.StringArray("bootnodes", []string{}, "list of bootnodes (multiaddress separated by commas)")
	bootnode := flag.Bool("bootnode", false, "act as a bootstrap node (will not try to connect with other bootnodes)")

	flag.Parse()
	log.Init(*logLevel, "stdout")
	ipfsStore := data.IPFSNewConfig(*dataDir)
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		log.Fatal(err)
	}

	var sk signature.SignKeys
	var privKey string

	if len(*nodeKey) > 0 {
		if err := sk.AddHexKey(*nodeKey); err != nil {
			log.Fatal(err)
		}
		_, privKey = sk.HexString()
	} else {
		pk := make([]byte, 64)
		kfile, err := os.OpenFile(*dataDir+"/.ipfsSync.key", os.O_CREATE|os.O_RDWR, 0600)
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
			if err := sk.AddHexKey(fmt.Sprintf("%s", pk)); err != nil {
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
	is.HelloTime = *helloTime
	is.UpdateTime = *updateTime
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
