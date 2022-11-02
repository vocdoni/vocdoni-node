package service

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/data/downloader"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/offchaindatahandler"
	"go.vocdoni.io/dvote/vochain/processarchive"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

type VochainService struct {
	Config         *config.VochainCfg
	App            *vochain.BaseApplication
	MetricsAgent   *metrics.Agent
	OffChainData   *offchaindatahandler.OffChainDataHandler
	DataDownloader *downloader.Downloader
	CensusDB       *censusdb.CensusDB
	Scrutinizer    *scrutinizer.Scrutinizer
	Stats          *vochaininfo.VochainInfo
	Storage        data.Storage
}

// NewVochainService initializes the Vochain service.  It takes the service configuration and
// initializes the missing services of the VochainService struct.  All started services will be
// respected unless the App, which is the main service and thus overwriten by this function.
func NewVochainService(vs *VochainService) error {
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
			if string(genesisBytes) != vochain.Genesis[vs.Config.Chain].Genesis {
				// if using a development chain, restore vochain
				if vochain.Genesis[vs.Config.Chain].AutoUpdateGenesis || vs.Config.Dev {
					log.Warn("new genesis found, cleaning and restarting Vochain")
					if err = os.RemoveAll(vs.Config.DataDir); err != nil {
						return err
					}
					if _, ok := vochain.Genesis[vs.Config.Chain]; !ok {
						err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Chain)
						return err
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
				return err
			}
			// If dev mode enabled, auto-update genesis file
			log.Debug("genesis does not exist, using hardcoded genesis")
			err = nil
			if _, ok := vochain.Genesis[vs.Config.Chain]; !ok {
				err = fmt.Errorf("cannot find a valid genesis for the %s network", vs.Config.Chain)
				return err
			}
			genesisBytes = []byte(vochain.Genesis[vs.Config.Chain].Genesis)
		}
	}

	// Metrics agent (Prometheus)
	if vs.MetricsAgent != nil {
		vs.Config.TendermintMetrics = true
	}

	// Create the vochain node
	vs.App = vochain.NewVochain(vs.Config, genesisBytes)

	// If seed mode, we are finished
	if vs.Config.IsSeedNode {
		return vs.App.Service.Start()
	}

	// Scrutinizer
	if vs.Config.Scrutinizer.Enabled && vs.Scrutinizer == nil {
		log.Info("creating vochain scrutinizer service")
		if vs.Scrutinizer, err = scrutinizer.NewScrutinizer(
			filepath.Join(vs.Config.DataDir, "scrutinizer"),
			vs.App,
			!vs.Config.Scrutinizer.IgnoreLiveResults,
		); err != nil {
			return err
		}
		// Launch the scrutinizer after sync routine (executed when the blockchain is ready)
		go vs.Scrutinizer.AfterSyncBootstrap()
	}

	// Data Downloader
	if vs.Config.OffChainDataDownloader && vs.OffChainData == nil {
		log.Infof("creating offchain data downloader service")
		if vs.DataDownloader == nil {
			vs.DataDownloader = downloader.NewDownloader(vs.Storage)
		}
		if vs.CensusDB == nil {
			db, err := metadb.New(db.TypePebble, filepath.Join(vs.Config.DataDir, "censusdb"))
			if err != nil {
				return err
			}
			vs.CensusDB = censusdb.NewCensusDB(db)
		}
		vs.OffChainData = offchaindatahandler.NewOffChainDataHandler(
			vs.App,
			vs.DataDownloader,
			vs.CensusDB,
			!vs.Config.ImportPreviousCensus,
		)
	}

	// Process Archiver
	if vs.Config.ProcessArchive {
		if vs.Scrutinizer == nil {
			err = fmt.Errorf("process archive needs indexer enabled")
			return err
		}
		ipfs, ok := vs.Storage.(*data.IPFSHandle)
		if !ok {
			log.Warnf("ipfsStorage is not IPFS, archive publishing disabled")
		}
		log.Infof("starting process archiver on %s", vs.Config.ProcessArchiveDataDir)
		processarchive.NewProcessArchive(
			vs.Scrutinizer,
			ipfs,
			vs.Config.ProcessArchiveDataDir,
			vs.Config.ProcessArchiveKey,
		)
	}

	// Vochain info
	if vs.Stats == nil {
		vs.Stats = vochaininfo.NewVochainInfo(vs.App)
		go vs.Stats.Start(10)

		// Grab metrics
		go vs.Stats.CollectMetrics(vs.MetricsAgent)
	}

	// Start the vochain node
	if err := vs.App.Service.Start(); err != nil {
		return err
	}

	if !vs.Config.NoWaitSync {
		log.Infof("waiting for vochain to synchronize")
		var lastHeight int64
		i := 0
		for !vs.Stats.Sync() {
			time.Sleep(time.Second * 1)
			if i%20 == 0 {
				log.Infof("[vochain info] fastsync running at height %d (%d blocks/s), peers %d",
					vs.Stats.Height(), (vs.Stats.Height()-lastHeight)/20, vs.Stats.NPeers())
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
