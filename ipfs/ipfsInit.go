package ipfs

import (
	"context"
	"io"
	"os"
	"path"
	"sync"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/assets"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/pkg/errors"

	"gitlab.com/vocdoni/go-dvote/log"
)

/*
func initWithDefaults(out io.Writer, repoRoot string, profile string) error {
	var profiles []string
	if profile != "" {
		profiles = strings.Split(profile, ",")
	}

	return doInit(out, repoRoot, false, nBitsForKeypairDefault)
}
*/

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
		return nil, errors.New("Repo exists!")
	}

	conf, err := config.Init(out, nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	// We don't need mdns, and it doesn't look like it works when not
	// running as root, anyway.
	conf.Discovery.MDNS.Enabled = false

	// Some optimizations from https://medium.com/coinmonks/ipfs-production-configuration-57121f0daab2
	conf.Datastore.BloomFilterSize = 1048576
	conf.Swarm.ConnMgr.LowWater = 100
	conf.Swarm.ConnMgr.HighWater = 400
	conf.Swarm.ConnMgr.GracePeriod = "20s"

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

func addDefaultAssets(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	dkey, err := assets.SeedInitDocs(nd)
	log.Errorf("init: seeding init docs failed: %s", err)

	log.Debugf("init: seeded init docs %s", dkey)

	log.Infof("to get started, enter:\n\tipfs cat /ipfs/%s/readme\n\n", dkey)
	return err
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	return namesys.InitializeKeyspace(ctx, nd.Namesys, nd.Pinning, nd.PrivateKey)
}
