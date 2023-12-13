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
// respected unless the App, which is the main service and thus overwritten by this function.
func (vs *VocdoniService) Vochain() error {
	// node + app layer
	if len(vs.Config.PublicAddr) == 0 {
		// tendermint doesn't have support for finding out it's own external address (as of v0.35.0)
		// there's some background discussion https://github.com/cometbft/cometbft/issues/758
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
		"network", vs.Config.Network,
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
			if !bytes.Equal(ethereum.HashRaw(genesisBytes), vocdoniGenesis.Genesis[vs.Config.Network].Genesis.Hash()) {
				// if auto-update genesis enabled, delete local genesis and use hardcoded genesis
				if vocdoniGenesis.Genesis[vs.Config.Network].AutoUpdateGenesis || vs.Config.Dev {
					log.Warn("new genesis found, cleaning and restarting Vochain")
					if err = os.RemoveAll(vs.Config.DataDir); err != nil {
						return err
					}
					if _, ok := vocdoniGenesis.Genesis[vs.Config.Network]; !ok {
						err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Network)
						return err
					}
					genesisBytes = vocdoniGenesis.Genesis[vs.Config.Network].Genesis.Marshal()
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
			if _, ok := vocdoniGenesis.Genesis[vs.Config.Network]; !ok {
				err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Network)
				return err
			}
			genesisBytes = vocdoniGenesis.Genesis[vs.Config.Network].Genesis.Marshal()
		}
	}

	// Create the vochain node
	vs.App = vochain.NewVochain(vs.Config, genesisBytes)

	return nil
}

// Start the vochain node and the vochaininfo service. Blocks until the node is ready if config.NoWaitSync is false.
func (vs *VocdoniService) Start() error {
	for {
		if err := vs.App.Node.Start(); err != nil {
			// failed to start PEX: address book is empty and couldn't resolve any seed nodes
			// may happen if the seed nodes are down or there are issues with the network.
			// So we retry until it works.
			if strings.Contains(err.Error(), "address book is empty") {
				log.Warn("address book is empty, retrying in 1 second")
				time.Sleep(time.Second * 1)
				continue
			}
			return err
		}
		break
	}
	// If seed mode, we are finished
	if vs.Config.IsSeedNode {
		return nil
	}
	// Vochain info
	if vs.Stats == nil {
		vs.Stats = vochaininfo.NewVochainInfo(vs.App)
		go vs.Stats.Start(10)
	}

	if !vs.Config.NoWaitSync {
		log.Infof("waiting for vochain to synchronize")
		var lastHeight uint64
		i := 0
		timeSyncCounter := time.Now()
		timeCounter := time.Now()
		syncCounter := 20
		for syncCounter > 0 {
			time.Sleep(time.Second * 1)
			if i%10 == 0 {
				log.Monitor("vochain fastsync", map[string]any{
					"height":     vs.Stats.Height(),
					"blocks/sec": fmt.Sprintf("%.2f", float64(vs.Stats.Height()-lastHeight)/time.Since(timeCounter).Seconds()),
					"peers":      vs.Stats.NPeers(),
				})
				timeCounter = time.Now()
				lastHeight = vs.Stats.Height()
			}
			i++
			if vs.App.IsSynchronizing() {
				syncCounter = 20
			} else {
				syncCounter--
			}
		}
		log.Infow("vochain fastsync completed", "height", vs.Stats.Height(), "time", time.Since(timeSyncCounter))
	}
	go VochainPrintInfo(20*time.Second, vs.Stats)

	if vs.Config.LogLevel == "debug" {
		go vochainPrintPeers(20*time.Second, vs.Stats)
	}
	return nil
}

// VochainPrintInfo initializes the Vochain statistics recollection.
func VochainPrintInfo(interval time.Duration, vi *vochaininfo.VochainInfo) {
	var h uint64
	var p, v uint64
	var m, vc, vxm uint64
	var b strings.Builder
	for {
		b.Reset()
		a := vi.BlockTimes()
		if a[1] > 0 {
			fmt.Fprintf(&b, "10m:%s", a[1].Truncate(time.Millisecond))
		}
		if a[2] > 0 {
			fmt.Fprintf(&b, " 1h:%s", a[2].Truncate(time.Millisecond))
		}
		if a[4] > 0 {
			fmt.Fprintf(&b, " 24h:%s", a[4].Truncate(time.Millisecond))
		}
		h = vi.Height()
		m = vi.MempoolSize()
		p, v, vxm = vi.TreeSizes()
		vc = vi.VoteCacheSize()
		log.Monitor("vochain status",
			map[string]any{
				"height":       h,
				"mempool":      m,
				"peers":        vi.NPeers(),
				"elections":    p,
				"votes":        v,
				"voteCache":    vc,
				"votes/min":    vxm,
				"blockPeriod":  b.String(),
				"blocksMinute": fmt.Sprintf("%.2f", vi.BlocksLastMinute()),
			})

		time.Sleep(interval)
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
		peers := make(map[string]any)
		for _, peer := range ni.Peers {
			peers[string(peer.NodeInfo.ID())] = peer.RemoteIP
		}
		log.Monitor("vochain peers", peers)
	}
}
