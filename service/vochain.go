package service

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
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
				return fmt.Errorf("invalid P2PListen %q: %w", vs.Config.P2PListen, err)
			}
			vs.Config.PublicAddr = net.JoinHostPort(ip.String(), port)
		}
	} else {
		host, port, err := net.SplitHostPort(vs.Config.PublicAddr)
		if err != nil {
			return fmt.Errorf("invalid PublicAddr %q: %w", vs.Config.PublicAddr, err)
		}
		vs.Config.PublicAddr = net.JoinHostPort(host, port)
	}
	log.Infow("vochain p2p enabled",
		"network", vs.Config.Network,
		"address", vs.Config.PublicAddr,
		"listenHost", vs.Config.P2PListen)

	// If genesis flag defined, use file as-is, never overwrite it, and fail if it doesn't exist
	if len(vs.Config.Genesis) > 0 {
		log.Infof("using custom genesis from %s", vs.Config.Genesis)
		if _, err := os.Stat(vs.Config.Genesis); os.IsNotExist(err) {
			return err
		}
	} else { // If genesis flag not defined, use local or hardcoded genesis
		vs.Config.Genesis = filepath.Join(vs.Config.DataDir, config.DefaultGenesisPath)

		if _, err := os.Stat(vs.Config.Genesis); err != nil {
			if os.IsNotExist(err) {
				log.Info("genesis file does not exist, will use hardcoded genesis")
			} else {
				return fmt.Errorf("unexpected error reading genesis: %w", err)
			}
		} else {
			log.Infof("found local genesis file at %s", vs.Config.Genesis)
			loadedGenesis, err := genesis.LoadFromFile(vs.Config.Genesis)
			if err != nil {
				return fmt.Errorf("couldn't parse local genesis file: %w", err)
			}
			hardcodedGenesis := genesis.HardcodedWithOverrides(vs.Config.Network,
				vs.Config.GenesisChainID,
				vs.Config.GenesisInitialHeight,
				vs.Config.GenesisAppHash)
			// compare genesis
			if loadedGenesis.ChainID != hardcodedGenesis.ChainID {
				log.Warnf("local genesis ChainID (%s) differs from hardcoded (%s)", loadedGenesis.ChainID, hardcodedGenesis.ChainID)
				if hardcodedGenesis.InitialHeight > 1 {
					log.Warnf("new hardcoded genesis creates a chain %q starting at block %d (i.e. on top of current %q), wiping out cometbft datadir",
						hardcodedGenesis.ChainID, hardcodedGenesis.InitialHeight, loadedGenesis.ChainID)
					if err = os.RemoveAll(filepath.Join(vs.Config.DataDir, config.DefaultCometBFTPath)); err != nil {
						return err
					}
				} else { // hardcodedGenesis.InitialHeight <= 0
					log.Warnf("new hardcoded genesis creates a chain %q from scratch, wiping out current chain %q datadir",
						hardcodedGenesis.ChainID, loadedGenesis.ChainID)
					if err = os.RemoveAll(vs.Config.DataDir); err != nil {
						return err
					}
				}

			}
		}
	}

	// Create the vochain node
	vs.App = vochain.NewVochain(vs.Config)

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

	if !vs.Config.NoWaitSync || vs.App.Node.Config().StateSync.Enable {
		log.Infof("waiting for vochain to synchronize")
		if vs.App.Node.Config().StateSync.Enable {
			log.Infof("temporarily setting comet loglevel to info, to see statesync progress")
			oldTenderLogLevel := log.CometLogLevel()
			log.SetCometLogLevel("info")
			defer func() {
				log.Infof("setting comet loglevel back to %q", oldTenderLogLevel)
				log.SetCometLogLevel(oldTenderLogLevel)
			}()
		}
		timeSyncStarted := time.Now()
		func() {
			lastHeight := uint64(0)
			timeCounter := time.Now()
			for {
				select {
				case <-vs.App.WaitUntilSynced():
					return
				case <-time.After(10 * time.Second):
					log.Monitor("vochain sync", map[string]any{
						"height":     vs.Stats.Height(),
						"blocks/sec": fmt.Sprintf("%.2f", float64(vs.Stats.Height()-lastHeight)/time.Since(timeCounter).Seconds()),
						"peers":      vs.Stats.NPeers(),
					})
					timeCounter = time.Now()
					lastHeight = vs.Stats.Height()
				}
			}
		}()
		log.Infow("vochain sync completed", "height", vs.Stats.Height(), "duration", time.Since(timeSyncStarted).String())
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
