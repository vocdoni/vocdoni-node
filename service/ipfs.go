package service

import (
	"os"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/data/ipfs/ipfsconnect"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// IPFS starts the IPFS service
func (vs *VocdoniService) IPFS(ipfsconfig *config.IPFSCfg) (data.Storage, error) {
	log.Info("creating ipfs service")
	if err := os.Setenv("IPFS_FD_MAX", "1024"); err != nil {
		log.Warnw("could not set IPFS_FD_MAX", "err", err)
	}
	storage := new(ipfs.Handler)
	if err := storage.Init(&types.DataStore{Datadir: ipfsconfig.ConfigPath}); err != nil {
		return nil, err
	}
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
			storage,
		)
		if len(ipfsconfig.ConnectPeers) > 0 && len(ipfsconfig.ConnectPeers[0]) > 8 {
			log.Debugf("using custom ipfsconnect bootnodes %s", ipfsconfig.ConnectPeers)
			ipfsconn.Transport.BootNodes = ipfsconfig.ConnectPeers
		}
		ipfsconn.Start()
	}
	return storage, nil
}
