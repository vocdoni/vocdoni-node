package ipfs

import (
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	config "github.com/ipfs/go-ipfs/config"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"go.vocdoni.io/dvote/log"
)

var pluginOnce sync.Once

func InstallDatabasePlugins() {
	pluginOnce.Do(func() {
		loader, err := loader.NewPluginLoader("")
		if err != nil {
			log.Fatal(err)
		}
		err = loader.Initialize()
		if err != nil {
			log.Fatal(err)
		}
		err = loader.Inject()
		if err != nil {
			log.Fatal(err)
		}
	})
}

func doInit(out io.Writer, repoRoot string, nBitsForKeypair int) (*config.Config, error) {
	log.Infof("initializing IPFS node at %s", repoRoot)

	if err := checkWritable(repoRoot); err != nil {
		return nil, err
	}

	if fsrepo.IsInitialized(repoRoot) {
		return nil, fmt.Errorf("repo exists")
	}

	conf, err := config.Init(out, nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	// Some optimizations to avoid using too much resources
	conf.Datastore.BloomFilterSize = 1 << 20 // 1MiB
	conf.Swarm.ConnMgr.LowWater = 20
	conf.Swarm.ConnMgr.HighWater = 100
	conf.Swarm.ConnMgr.GracePeriod = "2s"
	conf.Swarm.DisableBandwidthMetrics = true
	conf.Swarm.RelayClient.Enabled = config.False
	conf.Swarm.Transports.Network.Relay = 0
	conf.Datastore.GCPeriod = "5m"
	conf.Discovery.MDNS.Enabled = false

	// Prevent from scanning local networks which can trigger netscan alerts.
	// See: https://github.com/ipfs/go-ipfs/issues/7985
	conf.Swarm.AddrFilters = []string{
		"/ip4/10.0.0.0/ipcidr/8",
		"/ip4/100.64.0.0/ipcidr/10",
		"/ip4/169.254.0.0/ipcidr/16",
		"/ip4/172.16.0.0/ipcidr/12",
		"/ip4/192.0.0.0/ipcidr/24",
		"/ip4/192.0.2.0/ipcidr/24",
		"/ip4/192.168.0.0/ipcidr/16",
		"/ip4/198.18.0.0/ipcidr/15",
		"/ip4/198.51.100.0/ipcidr/24",
		"/ip4/203.0.113.0/ipcidr/24",
		"/ip4/240.0.0.0/ipcidr/4",
		"/ip6/100::/ipcidr/64",
		"/ip6/2001:2::/ipcidr/48",
		"/ip6/2001:db8::/ipcidr/32",
		"/ip6/fc00::/ipcidr/7",
		"/ip6/fe80::/ipcidr/10",
	}

	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return nil, err
	}
	log.Info("IPFS configuration file initialized")
	return conf, nil
}

func checkWritable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesn't exist, check that we can create it
		return os.Mkdir(dir, 0o775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}
