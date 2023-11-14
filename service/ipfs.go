package service

import (
	"os"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/data/ipfs/ipfsconnect"
	"go.vocdoni.io/dvote/log"
)

// IPFS starts the IPFS service
func IPFS(ipfsconfig *config.IPFSCfg) (storage data.Storage, err error) {
	log.Info("creating ipfs service")
	os.Setenv("IPFS_FD_MAX", "1024")

	ipfsStore := ipfs.New()
	ipfsStore.DataDir = ipfsconfig.ConfigPath
	ipfsStore.EnableLocalDiscovery = ipfsconfig.LocalDiscovery
	err = ipfsStore.Init()
	if err != nil {
		return nil, err
	}
	storage = ipfsStore

	go func() {
		for {
			time.Sleep(time.Second * 120)
			log.Monitor("ipfs storage", storage.Stats())
		}
	}()

	if len(ipfsconfig.ConnectKey) > 0 {
		log.Infow("starting ipfsconnect service", "key", ipfsconfig.ConnectKey)
		ipfsconn := ipfsconnect.New(
			ipfsconfig.ConnectKey,
			storage.(*ipfs.Handler),
		)
		if len(ipfsconfig.ConnectPeers) > 0 && len(ipfsconfig.ConnectPeers[0]) > 8 {
			log.Debugf("using custom ipfsconnect bootnodes %s", ipfsconfig.ConnectPeers)
			ipfsconn.Transport.BootNodes = ipfsconfig.ConnectPeers
		}
		ipfsconn.Start()
	}
	return storage, nil
}
