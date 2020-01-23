package ipfs

import (
	"io"
	"os"
	"path"
	"sync"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/pkg/errors"

	"gitlab.com/vocdoni/go-dvote/log"
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
		return nil, errors.New("repo exists")
	}

	conf, err := config.Init(out, nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	// Some optimizations to avoid using too much resources
	conf.Datastore.BloomFilterSize = 1048576
	conf.Swarm.ConnMgr.LowWater = 20
	conf.Swarm.ConnMgr.HighWater = 100
	conf.Swarm.ConnMgr.GracePeriod = "2s"
	conf.Swarm.DisableBandwidthMetrics = true
	conf.Swarm.EnableAutoRelay = false
	conf.Swarm.DisableRelay = true
	conf.Datastore.GCPeriod = "5m"
	conf.Discovery.MDNS.Enabled = false

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
				return errors.Errorf("%s is not writeable by the current user", dir)
			}
			return errors.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesn't exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return errors.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}
