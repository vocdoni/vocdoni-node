package service

import (
	"fmt"
	"net"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

func Vochain(vconfig *config.VochainCfg, dev, results bool) (vnode *vochain.BaseApplication, sc *scrutinizer.Scrutinizer, vs *types.VochainStats, err error) {
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
		host, port, err = net.SplitHostPort(vconfig.P2PListen)
		if err != nil {
			return
		}
		vconfig.PublicAddr = net.JoinHostPort(host, port)
	}
	log.Infof("vochain listening on: %s", vconfig.P2PListen)
	log.Infof("vochain exposed IP address: %s", vconfig.PublicAddr)

	if dev {
		vnode = vochain.NewVochain(vconfig, []byte(vochain.DevelopmentGenesis1))
	} else {
		vnode = vochain.NewVochain(vconfig, []byte(vochain.TestnetGenesis1))
	}
	// Scrutinizer
	if results {
		log.Info("creating vochain scrutinizer service")
		sc, err = scrutinizer.NewScrutinizer(vconfig.DataDir+"/scrutinizer", vnode.State)
		if err != nil {
			return
		}
	}
	vs = new(types.VochainStats)
	go VochainStatsCollect(vnode, 20, vs)
	return
}

// VochainStatsCollect initializes the Vochain statistics recollection
func VochainStatsCollect(vnode *vochain.BaseApplication, sleepSecs int64, vs *types.VochainStats) {
	var pheight int64
	var h1, h10, h60, h360 int64
	var n1, n10, n60, n360 int64
	for {
		// TODO(mvdan): this seems racy too
		if vnode.Node == nil {
			time.Sleep(time.Duration(sleepSecs) * time.Second)
			continue
		}
		var b strings.Builder
		// TODO(mvdan): this seems racy with other goroutines reading
		// from VochainStats
		vs.Height = vnode.Node.BlockStore().Height()
		// less than 2s per block it's not real. Consider blockchain is synchcing
		if pheight > 0 && sleepSecs/2 > (vs.Height-pheight) {
			vs.Sync = true
			n1++
			n10++
			n60++
			n360++
			h1 += (vs.Height - pheight)
			h10 += (vs.Height - pheight)
			h60 += (vs.Height - pheight)
			h360 += (vs.Height - pheight)
			if sleepSecs*n1 >= 60 && h1 > 0 {
				vs.Avg1 = (n1 * sleepSecs) / h1
				n1 = 0
				h1 = 0
			}
			if sleepSecs*n10 >= 600 && h10 > 0 {
				vs.Avg10 = (n10 * sleepSecs) / h10
				n10 = 0
				h10 = 0
			}
			if sleepSecs*n60 >= 3600 && h60 > 0 {
				vs.Avg60 = (n60 * sleepSecs) / h60
				n60 = 0
				h60 = 0
			}
			if sleepSecs*n360 >= 21600 && h360 > 0 {
				vs.Avg360 = (n360 * sleepSecs) / h360
				n360 = 0
				h360 = 0
			}
			if vs.Avg1 > 0 {
				fmt.Fprintf(&b, "1m:%d", vs.Avg1)
			}
			if vs.Avg10 > 0 {
				fmt.Fprintf(&b, " 10m:%d", vs.Avg10)
			}
			if vs.Avg60 > 0 {
				fmt.Fprintf(&b, " 1h:%d", vs.Avg60)
			}
			if vs.Avg360 > 0 {
				fmt.Fprintf(&b, " 6h:%d", vs.Avg360)
			}
		} else {
			vs.Sync = false
		}
		pheight = vs.Height
		vs.ProcessTreeSize = vnode.State.ProcessTree.Size()
		vs.VoteTreeSize = vnode.State.VoteTree.Size()
		vs.MempoolSize = vnode.Node.Mempool().Size()
		log.Infof("[vochain info] height:%d mempool:%d processTree:%d voteTree:%d blockTime:{%s}",
			vs.Height,
			vs.MempoolSize,
			vs.ProcessTreeSize,
			vs.VoteTreeSize,
			b,
		)
		time.Sleep(time.Duration(sleepSecs) * time.Second)
	}
}
