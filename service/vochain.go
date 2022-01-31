package service

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/censusdownloader"
	"go.vocdoni.io/dvote/vochain/processarchive"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

type VochainService struct {
	Config        *config.VochainCfg
	MetricsAgent  *metrics.Agent
	CensusManager *census.Manager
	Storage       data.Storage
}

// Vochain creates a new vochain service
func Vochain(vs *VochainService) (
	*vochain.BaseApplication, *scrutinizer.Scrutinizer,
	*vochaininfo.VochainInfo, error) {
	log.Infof("creating vochain service for network %s", vs.Config.Chain)
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
				return nil, nil, nil, err
			}
			vs.Config.PublicAddr = net.JoinHostPort(ip.String(), port)
		}
	} else {
		host, port, err := net.SplitHostPort(vs.Config.PublicAddr)
		if err != nil {
			return nil, nil, nil, err
		}
		vs.Config.PublicAddr = net.JoinHostPort(host, port)
	}
	log.Infof("vochain listening on: %s", vs.Config.P2PListen)
	log.Infof("vochain exposed IP address: %s", vs.Config.PublicAddr)
	log.Infof("vochain RPC listening on: %s", vs.Config.RPCListen)

	// Genesis file
	var genesisBytes []byte
	var err error

	// If genesis file from flag, prioritize it
	if len(vs.Config.Genesis) > 0 {
		log.Infof("using custom genesis from %s", vs.Config.Genesis)
		if _, err := os.Stat(vs.Config.Genesis); os.IsNotExist(err) {
			return nil, nil, nil, err
		}
		if genesisBytes, err = os.ReadFile(vs.Config.Genesis); err != nil {
			return nil, nil, nil, err
		}
	} else { // If genesis flag not defined, use a hardcoded or local genesis
		genesisBytes, err = os.ReadFile(vs.Config.DataDir + "/config/genesis.json")

		if err == nil { // If genesis file found
			log.Info("found genesis file")
			// compare genesis
			if string(genesisBytes) != vochain.Genesis[vs.Config.Chain].Genesis {
				// if using a development chain, restore vochain
				if vochain.Genesis[vs.Config.Chain].AutoUpdateGenesis || vs.Config.Dev {
					log.Warn("new genesis found, cleaning and restarting Vochain")
					if err = os.RemoveAll(vs.Config.DataDir); err != nil {
						return nil, nil, nil, err
					}
					if _, ok := vochain.Genesis[vs.Config.Chain]; !ok {
						err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Chain)
						return nil, nil, nil, err
					}
					genesisBytes = []byte(vochain.Genesis[vs.Config.Chain].Genesis)
				} else {
					log.Warn("local and hardcoded genesis are different, risk of potential consensus failure")
				}
			} else {
				log.Info("local genesis matches with the hardcoded genesis")
			}
		} else { // If genesis file not found
			if !os.IsNotExist(err) {
				return nil, nil, nil, err
			}
			// If dev mode enabled, auto-update genesis file
			log.Debug("genesis does not exist, using hardcoded genesis")
			err = nil
			if _, ok := vochain.Genesis[vs.Config.Chain]; !ok {
				err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Chain)
				return nil, nil, nil, err
			}
			genesisBytes = []byte(vochain.Genesis[vs.Config.Chain].Genesis)
		}
	}

	// Metrics agent (Prometheus)
	if vs.MetricsAgent != nil {
		vs.Config.TendermintMetrics = true
	}

	// Create the vochain node
	vnode := vochain.NewVochain(vs.Config, genesisBytes)

	// If seed mode, we are finished
	if vs.Config.IsSeedNode {
		return vnode, nil, nil, vnode.Service.Start()
	}

	// Scrutinizer
	var sc *scrutinizer.Scrutinizer
	if vs.Config.Scrutinizer.Enabled {
		log.Info("creating vochain scrutinizer service")
		if sc, err = scrutinizer.NewScrutinizer(
			filepath.Join(vs.Config.DataDir, "scrutinizer"),
			vnode,
			!vs.Config.Scrutinizer.IgnoreLiveResults,
		); err != nil {
			return nil, nil, nil, err
		}
		go sc.AfterSyncBootstrap()
	}

	// Census Downloader
	if vs.CensusManager != nil {
		log.Infof("starting census downloader service")
		censusdownloader.NewCensusDownloader(vnode, vs.CensusManager, !vs.Config.ImportPreviousCensus)
	}

	// Process Archiver
	if vs.Config.ProcessArchive {
		if sc == nil {
			err = fmt.Errorf("process archive needs indexer enabled")
			return nil, nil, nil, err
		}
		ipfs, ok := vs.Storage.(*data.IPFSHandle)
		if !ok {
			log.Warnf("ipfsStorage is not IPFS, archive publishing disabled")
		}
		log.Infof("starting process archiver on %s", vs.Config.ProcessArchiveDataDir)
		processarchive.NewProcessArchive(
			sc,
			ipfs,
			vs.Config.ProcessArchiveDataDir,
			vs.Config.ProcessArchiveKey,
		)
	}

	// Vochain info
	vi := vochaininfo.NewVochainInfo(vnode)
	go vi.Start(10)

	// Grab metrics
	go vi.CollectMetrics(vs.MetricsAgent)

	// Start the vochain node
	if err := vnode.Service.Start(); err != nil {
		return nil, nil, nil, err
	}

	if !vs.Config.NoWaitSync {
		log.Infof("waiting for vochain to synchronize")
		var lastHeight int64
		i := 0
		for !vi.Sync() {
			time.Sleep(time.Second * 1)
			if i%20 == 0 {
				log.Infof("[vochain info] fastsync running at height %d (%d blocks/s), peers %d",
					vi.Height(), (vi.Height()-lastHeight)/20, vi.NPeers())
				lastHeight = vi.Height()
			}
			i++
		}
		log.Infof("vochain fastsync completed!")
	}
	go VochainPrintInfo(20, vi)

	if vs.Config.LogLevel == "debug" {
		go vochainPrintPeers(20*time.Second, vi)
	}

	return vnode, sc, vi, nil
}

// VochainPrintInfo initializes the Vochain statistics recollection
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
		log.Infof("[vochain info] height:%d mempool:%d peers:%d "+
			"processes:%d votes:%d vxm:%d voteCache:%d blockTime:{%s}",
			h, m, vi.NPeers(), p, v, vxm, vc, b.String(),
		)
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
		for _, peer := range ni.Peers {
			log.Debugf("vochain peers: %s", peer.URL)
		}
	}
}
