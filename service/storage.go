package service

import (
	"context"
	"os"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/ipfssync"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
)

func IPFS(ipfsconfig *config.IPFSCfg, signer *ethereum.SignKeys,
	ma *metrics.Agent) (storage data.Storage, err error) {
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
				time.Sleep(time.Second * 120)
				tctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				stats, err := storage.Stats(tctx)
				cancel()
				if err != nil {
					log.Warnf("IPFS node returned an error: %s", err)
				}
				log.Infof("[ipfs info] %s", stats)
			}
		}()

		go storage.CollectMetrics(context.Background(), ma)

		if len(ipfsconfig.SyncKey) > 0 {
			log.Info("enabling ipfs synchronization")
			_, priv := signer.HexString()
			storageSync = *ipfssync.NewIPFSsync(ipfsconfig.ConfigPath+"/.ipfsSync", ipfsconfig.SyncKey, priv, "libp2p", storage)
			if len(ipfsconfig.SyncPeers) > 0 && len(ipfsconfig.SyncPeers[0]) > 8 {
				log.Debugf("using custom ipfs sync bootnodes %s", ipfsconfig.SyncPeers)
				storageSync.Transport.BootNodes = ipfsconfig.SyncPeers
			}
			if ipfsconfig.SyncLogLevel != "" {
				storageSync.SetLogger(log.LoggerWithLevel(ipfsconfig.SyncLogLevel))
			} else {
				storageSync.SetLogger(log.Logger())
			}
			storageSync.Start()
		}
	}
	return
}
