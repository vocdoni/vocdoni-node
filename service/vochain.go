package service

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/censusdownloader"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
	"gitlab.com/vocdoni/go-dvote/vochain/vochaininfo"
)

func Vochain(vconfig *config.VochainCfg, dev, results, waitForSync bool, ma *metrics.Agent, cm *census.Manager) (vnode *vochain.BaseApplication, sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo, err error) {
	log.Info("creating vochain service")
	var host, port string
	var ip net.IP
	// node + app layer
	if len(vconfig.PublicAddr) == 0 {
		ip, err = util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			_, port, err = net.SplitHostPort(vconfig.P2PListen)
			if err != nil {
				return
			}
			vconfig.PublicAddr = net.JoinHostPort(ip.String(), port)
		}
	} else {
		host, port, err = net.SplitHostPort(vconfig.PublicAddr)
		if err != nil {
			return
		}
		vconfig.PublicAddr = net.JoinHostPort(host, port)
	}
	log.Infof("vochain listening on: %s", vconfig.P2PListen)
	log.Infof("vochain exposed IP address: %s", vconfig.PublicAddr)
	log.Infof("vochain RPC listening on: %s", vconfig.RPCListen)
	// Genesis file
	var genesisBytes []byte

	// If genesis file from flag, prioritize it
	if len(vconfig.Genesis) > 0 {
		log.Infof("using custom genesis from %s", vconfig.Genesis)
		if _, err = os.Stat(vconfig.Genesis); os.IsNotExist(err) {
			return
		}
		if genesisBytes, err = ioutil.ReadFile(vconfig.Genesis); err != nil {
			return
		}
	} else {
		// If genesis flag not defined, use either dev or release genesis
		if dev {
			// If dev mode enabled, auto-update genesis file
			filepath := vconfig.DataDir + "/config/genesis.json"
			if _, err = os.Stat(filepath); os.IsNotExist(err) {
				log.Debug("genesis does not exist, using hardcoded genesis")
				err = nil
			} else {
				if genesisBytes, err = ioutil.ReadFile(filepath); err != nil {
					return
				}
				log.Debug("found genesis file, comparing with hardcoded genesis")
				// compare genesis
				if string(genesisBytes) != vochain.DevelopmentGenesis1 {
					log.Warn("genesis found is different from the hardcoded genesis, cleaning and restarting vochain")
					if err = os.RemoveAll(vconfig.DataDir); err != nil {
						return
					}
				} else {
					log.Debug("genesis is updated")
				}
			}
			genesisBytes = []byte(vochain.DevelopmentGenesis1)
		} else {
			genesisBytes = []byte(vochain.ReleaseGenesis1)
		}
	}

	if ma != nil {
		vconfig.TendermintMetrics = true
	}

	vnode = vochain.NewVochain(vconfig, genesisBytes)
	// Scrutinizer
	if results {
		log.Info("creating vochain scrutinizer service")
		sc, err = scrutinizer.NewScrutinizer(vconfig.DataDir+"/scrutinizer", vnode.State)
		if err != nil {
			return
		}
	}
	if cm != nil {
		log.Infof("starting census downloader service")
		censusdownloader.NewCensusDownloader(vnode, cm, !vconfig.ImportPreviousCensus)
	}
	// Grab metrics
	go vnode.CollectMetrics(ma)

	// Vochain info
	vi = vochaininfo.NewVochainInfo(vnode)
	go vi.Start(60)

	if waitForSync && !vconfig.SeedMode {
		log.Infof("waiting for vochain to synchronize")
		var lastHeight int64
		i := 0
		for !vi.Sync() {
			time.Sleep(time.Second * 1)
			if i%20 == 0 {
				log.Infof("[vochain info] fastsync running at block %d (%d blocks/s), peers %d", vi.Height(), (vi.Height()-lastHeight)/20, len(vi.Peers()))
				lastHeight = vi.Height()
			}
			i++
		}
		log.Infof("vochain fastsync completed!")
	}
	go VochainPrintInfo(20, vi)

	return
}

// VochainPrintInfo initializes the Vochain statistics recollection
func VochainPrintInfo(sleepSecs int64, vi *vochaininfo.VochainInfo) {
	var a *[5]int32
	var h, p, v int64
	var m, vc int
	var b strings.Builder
	for {
		b.Reset()
		a = vi.BlockTimes()
		if a[0] > 0 {
			fmt.Fprintf(&b, "1m:%.2f", float32(a[0]/1000))
		}
		if a[1] > 0 {
			fmt.Fprintf(&b, " 10m:%.2f", float32(a[1]/1000))
		}
		if a[2] > 0 {
			fmt.Fprintf(&b, " 1h:%.2f", float32(a[2]/1000))
		}
		if a[3] > 0 {
			fmt.Fprintf(&b, " 6h:%.2f", float32(a[3]/1000))
		}
		if a[4] > 0 {
			fmt.Fprintf(&b, " 24h:%.2f", float32(a[4]/1000))
		}
		h = vi.Height()
		m = vi.MempoolSize()
		p, v = vi.TreeSizes()
		vc = vi.VoteCacheSize()
		log.Infof("[vochain info] height:%d mempool:%d peers:%d processTree:%d voteTree:%d voteCache:%d blockTime:{%s}",
			h, m, len(vi.Peers()), p, v, vc, b.String(),
		)
		time.Sleep(time.Duration(sleepSecs) * time.Second)
	}
}
