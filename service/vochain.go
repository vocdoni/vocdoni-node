package service

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	vocdoniGenesis "go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// Vochain initializes the Vochain service.  It takes the service configuration and
// initializes the missing services of the VocdoniService struct.  All started services will be
// respected unless the App, which is the main service and thus overwriten by this function.
func (vs *VocdoniService) Vochain() error {
	// node + app layer
	if len(vs.Config.PublicAddr) == 0 {
		// tendermint doesn't have support for finding out it's own external address (as of v0.35.0)
		// there's some background discussion https://github.com/tendermint/tendermint/issues/758
		// so we need to find it ourselves out somehow
		// (PublicAddr ends up being passed to tendermint as ExternalAddress)
		ip, err := util.PublicIP(4)
		if err != nil {
			log.Warn(err)
		} else {
			_, port, err := net.SplitHostPort(vs.Config.P2PListen)
			if err != nil {
				return err
			}
			vs.Config.PublicAddr = net.JoinHostPort(ip.String(), port)
		}
	} else {
		host, port, err := net.SplitHostPort(vs.Config.PublicAddr)
		if err != nil {
			return err
		}
		vs.Config.PublicAddr = net.JoinHostPort(host, port)
	}
	log.Infow("vochain p2p enabled",
		"network", vs.Config.Chain,
		"address", vs.Config.PublicAddr,
		"listenHost", vs.Config.P2PListen)

	// Genesis file
	var genesisBytes []byte
	var err error
	// If genesis file from flag, prioritize it
	if len(vs.Config.Genesis) > 0 {
		log.Infof("using custom genesis from %s", vs.Config.Genesis)
		if _, err := os.Stat(vs.Config.Genesis); os.IsNotExist(err) {
			return err
		}
		if genesisBytes, err = os.ReadFile(vs.Config.Genesis); err != nil {
			return err
		}
	} else { // If genesis flag not defined, use a hardcoded or local genesis
		genesisBytes, err = os.ReadFile(vs.Config.DataDir + "/config/genesis.json")

		if err == nil { // If genesis file found
			log.Info("found genesis file")
			// compare genesis
			if !bytes.Equal(ethereum.HashRaw(genesisBytes), vocdoniGenesis.Genesis[vs.Config.Chain].Genesis.Hash()) {
				// if auto-update genesis enabled, delete local genesis and use hardcoded genesis
				if vocdoniGenesis.Genesis[vs.Config.Chain].AutoUpdateGenesis || vs.Config.Dev {
					log.Warn("new genesis found, cleaning and restarting Vochain")
					if err = os.RemoveAll(vs.Config.DataDir); err != nil {
						return err
					}
					if _, ok := vocdoniGenesis.Genesis[vs.Config.Chain]; !ok {
						err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Chain)
						return err
					}
					genesisBytes = vocdoniGenesis.Genesis[vs.Config.Chain].Genesis.Marshal()
				} else {
					log.Warn("local and hardcoded genesis are different, risk of potential consensus failure")
				}
			} else {
				log.Info("local and factory genesis match")
			}
		} else { // If genesis file not found
			if !os.IsNotExist(err) {
				return err
			}
			log.Info("genesis file does not exist, using factory")
			if _, ok := vocdoniGenesis.Genesis[vs.Config.Chain]; !ok {
				err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Chain)
				return err
			}
			genesisBytes = vocdoniGenesis.Genesis[vs.Config.Chain].Genesis.Marshal()
		}
	}

	// Metrics agent (Prometheus)
	if vs.MetricsAgent != nil {
		vs.Config.TendermintMetrics = true
	}

	// Create the vochain node
	vs.App = vochain.NewVochain(vs.Config, genesisBytes)

	return nil
}

// Start the vochain node and the vochaininfo service. Blocks until the node is ready if config.NoWaitSync is false.
func (vs *VocdoniService) Start() error {
	if err := vs.App.Service.Start(); err != nil {
		return err
	}
	// If seed mode, we are finished
	if vs.Config.IsSeedNode {
		return nil
	}
	// Vochain info
	if vs.Stats == nil {
		vs.Stats = vochaininfo.NewVochainInfo(vs.App)
		go vs.Stats.Start(10)

		// Grab metrics
		go vs.Stats.CollectMetrics(vs.MetricsAgent)
	}

	if !vs.Config.NoWaitSync {
		log.Infof("waiting for vochain to synchronize")
		var lastHeight int64
		i := 0
		timeCounter := time.Now()
		for !vs.Stats.Sync() {
			time.Sleep(time.Second * 1)
			if i%10 == 0 {
				log.Monitor("vochain fastsync", map[string]interface{}{
					"height":   vs.Stats.Height(),
					"blocks/s": float64((vs.Stats.Height() - lastHeight)) / time.Since(timeCounter).Seconds(),
					"peers":    vs.Stats.NPeers(),
				})
				timeCounter = time.Now()
				lastHeight = vs.Stats.Height()
			}
			i++
		}
		log.Infof("vochain fastsync completed!")
	}
	go VochainPrintInfo(20, vs.Stats)

	if vs.Config.LogLevel == "debug" {
		go vochainPrintPeers(20*time.Second, vs.Stats)
	}
	return nil
}

// VochainPrintInfo initializes the Vochain statistics recollection.
func VochainPrintInfo(sleepSecs int64, vi *vochaininfo.VochainInfo) {
	var a *[5]int32
	var h int64
	var p, v uint64
	var m, vc, vxm int
	var b strings.Builder
	for {
		b.Reset()
		a = vi.BlockTimes()
		if a[0] > 0 {
			fmt.Fprintf(&b, "1m:%.2f", float32(a[0])/1000)
		}
		if a[1] > 0 {
			fmt.Fprintf(&b, " 10m:%.2f", float32(a[1])/1000)
		}
		if a[2] > 0 {
			fmt.Fprintf(&b, " 1h:%.2f", float32(a[2])/1000)
		}
		if a[3] > 0 {
			fmt.Fprintf(&b, " 6h:%.2f", float32(a[3])/1000)
		}
		if a[4] > 0 {
			fmt.Fprintf(&b, " 24h:%.2f", float32(a[4])/1000)
		}
		h = vi.Height()
		m = vi.MempoolSize()
		p, v, vxm = vi.TreeSizes()
		vc = vi.VoteCacheSize()
		log.Monitor("vochain status",
			map[string]interface{}{
				"height":    h,
				"mempool":   m,
				"peers":     vi.NPeers(),
				"elections": p,
				"votes":     v,
				"voteCache": vc,
				"votes/min": vxm,
				"blockTime": b.String(),
			})

		time.Sleep(time.Duration(sleepSecs) * time.Second)
	}
}

// vochainPrintPeers prints the peers to log, for debugging tendermint PEX
func vochainPrintPeers(interval time.Duration, vi *vochaininfo.VochainInfo) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		<-t.C
		ni, err := vi.NetInfo()
		if err != nil {
			log.Warn(err)
			continue
		}
		peers := make(map[string]interface{})
		for _, peer := range ni.Peers {
			peers[peer.ID.AddressString("")] = peer.URL
		}
		log.Monitor("vochain peers", peers)
	}
}
