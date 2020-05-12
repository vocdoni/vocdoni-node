package service

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	voclient "github.com/tendermint/tendermint/rpc/client"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
	"gitlab.com/vocdoni/go-dvote/vochain/vochaininfo"
)

func Vochain(vconfig *config.VochainCfg, dev, results bool, ma *metrics.Agent) (vnode *vochain.BaseApplication, sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo, err error) {
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

	if dev {
		filepath := vconfig.DataDir + "/config/genesis.json"
		if _, err = os.Stat(filepath); os.IsNotExist(err) {
			log.Debug("genesis does not exist, using hardcoded genesis")
		} else {
			var genesisBytes []byte
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
				log.Debug("genesis are the same, you have the latest dev genesis")
			}
		}
		vnode = vochain.NewVochain(vconfig, []byte(vochain.DevelopmentGenesis1))
	} else {
		vnode = vochain.NewVochain(vconfig, []byte(vochain.ReleaseGenesis1))
	}
	// Scrutinizer
	if results {
		log.Info("creating vochain scrutinizer service")
		sc, err = scrutinizer.NewScrutinizer(vconfig.DataDir+"/scrutinizer", vnode.State, false)
		if err != nil {
			return
		}
	}
	// Grab metrics
	go vnode.CollectMetrics(ma)

	// Vochain info
	vi = vochaininfo.NewVochainInfo(vnode)
	go vi.Start(10)
	go VochainPrintInfo(20, vi)

	// Vochain RPC client
	vnode.Client, err = voclient.NewHTTP("tcp://"+vconfig.RPCListen, "/websocket")
	go voclientCheck(vnode.Client, vconfig.RPCListen)

	return
}

// VochainStatsCollect initializes the Vochain statistics recollection
func VochainPrintInfo(sleepSecs int64, vi *vochaininfo.VochainInfo) {
	var a1, a10, a60, a360, a1440 float32
	var h, p, v int64
	var m int
	var b strings.Builder
	for {
		b.Reset()
		a1, a10, a60, a360, a1440 = vi.BlockTimes()
		if a1 > 0 {
			fmt.Fprintf(&b, "1m:%.1f", a1)
		}
		if a10 > 0 {
			fmt.Fprintf(&b, " 10m:%.1f", a10)
		}
		if a60 > 0 {
			fmt.Fprintf(&b, " 1h:%.1f", a60)
		}
		if a360 > 0 {
			fmt.Fprintf(&b, " 6h:%.1f", a360)
		}
		if a1440 > 0 {
			fmt.Fprintf(&b, " 24h:%.1f", a1440)
		}
		h = vi.Height()
		m = vi.MempoolSize()
		p, v = vi.TreeSizes()
		log.Infof("[vochain info] height:%d mempool:%d processTree:%d voteTree:%d blockTime:{%s}",
			h, m, p, v, b.String(),
		)
		time.Sleep(time.Duration(sleepSecs) * time.Second)
	}
}

// Try to keep the RPC connection alive
// Dislaimer: not sure if this mechanism really works, should be deeply checked
func voclientCheck(vc *voclient.HTTP, rpc string) {
	for {
		if s, err := vc.Status(); s == nil || err != nil {
			log.Warnf("tendermint RPC is dead, trying to recover")
			vc, err = voclient.NewHTTP("tcp://"+rpc, "/websocket")
			if err != nil {
				log.Errorf("tendermint RPC connection cannot be recovered")
			}
		}
		time.Sleep(time.Second * 2)
	}
}
