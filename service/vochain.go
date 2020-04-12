package service

import (
	"net"
	"time"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

func Vochain(vconfig *config.VochainCfg, dev, results bool) (vnode *vochain.BaseApplication, sc *scrutinizer.Scrutinizer, err error) {
	log.Info("creating vochain service")

	// node + app layer
	if len(vconfig.PublicAddr) == 0 {
		ip, err := util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			_, port, err := net.SplitHostPort(vconfig.P2PListen)
			if err == nil {
				vconfig.PublicAddr = net.JoinHostPort(ip.String(), port)
			}
		}
	} else {
		host, port, err := net.SplitHostPort(vconfig.P2PListen)
		if err == nil {
			vconfig.PublicAddr = net.JoinHostPort(host, port)
		}
	}
	if vconfig.PublicAddr != "" {
		log.Infof("vochain exposed IP address: %s", vconfig.PublicAddr)
	}

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

	// Stats info
	go func() {
		for {
			if vnode.Node != nil {
				log.Infof("[vochain info] height:%d mempool:%d appTree:%d processTree:%d voteTree:%d",
					vnode.Node.BlockStore().Height(),
					vnode.Node.Mempool().Size(),
					vnode.State.AppTree.Size(),
					vnode.State.ProcessTree.Size(),
					vnode.State.VoteTree.Size(),
				)
			}
			time.Sleep(20 * time.Second)
		}
	}()
	defer func() {
		vnode.Node.Stop()
		vnode.Node.Wait()
	}()
	return
}
