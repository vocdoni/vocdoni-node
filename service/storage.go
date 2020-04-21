package service

import (
	"context"
	"os"
	"time"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/ipfssync"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
)

func IPFS(ipfsconfig *config.IPFSCfg, signer *signature.SignKeys, ma *metrics.Agent) (storage data.Storage, err error) {
	log.Info("creating ipfs service")
	var storageSync ipfssync.IPFSsync
	if !ipfsconfig.NoInit {
		os.Setenv("IPFS_FD_MAX", "1024")
		ipfsStore := data.IPFSNewConfig(ipfsconfig.ConfigPath)
		storage, err = data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
		if err != nil {
			return
		}

		go func() {
			for {
				time.Sleep(time.Second * 20)
				stats, err := storage.Stats(context.TODO())
				if err != nil {
					log.Warnf("IPFS node returned an error: %s", err)
				}
				log.Infof("[ipfs info] %s", stats)
			}
		}()

		if ma != nil {
			storage.RegisterMetrics(ma)
			go func() {
				for {
					time.Sleep(ma.RefreshInterval)
					err := storage.GetMetrics(context.TODO())
					if err != nil {
						log.Warnf("IPFS metrics returned an error: %s", err)
					}
				}
			}()
		}

		if len(ipfsconfig.SyncKey) > 0 {
			log.Info("enabling ipfs synchronization")
			_, priv := signer.HexString()
			storageSync = *ipfssync.NewIPFSsync(ipfsconfig.ConfigPath+"/.ipfsSync", ipfsconfig.SyncKey, priv, "libp2p", storage)
			if len(ipfsconfig.SyncPeers) > 0 && len(ipfsconfig.SyncPeers[0]) > 8 {
				log.Debugf("using custom ipfs sync bootnodes %s", ipfsconfig.SyncPeers)
				storageSync.Transport.SetBootnodes(ipfsconfig.SyncPeers)
			}
			go storageSync.Start()
		}
	}
	return
}
