package ipfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	coreiface "github.com/ipfs/boxo/coreiface"
	ihelper "github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	ipfscid "github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/commands"
	config "github.com/ipfs/kubo/config"
	ipfscore "github.com/ipfs/kubo/core"
	ipfsapi "github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"

	"go.vocdoni.io/dvote/log"
)

var (
	pluginOnce sync.Once
	ConfigRoot string
)

const (
	// ChunkerTypeSize is the chunker type used by IPFS to calculate to build the DAG.
	ChunkerTypeSize = "size-262144"
)

func init() {
	// Initialize the DAG builder with offline exchange and the correct CID format
	format := ipfscid.V1Builder{
		Codec:  uint64(multicodec.DagJson),
		MhType: uint64(multihash.SHA2_256),
	}
	dAGbuilder = ihelper.DagBuilderParams{
		Dagserv:    dAG(),
		RawLeaves:  false,
		Maxlinks:   ihelper.DefaultLinksPerBlock,
		NoCopy:     false,
		CidBuilder: &format,
	}
}

// Init initializes the IPFS node and repository.
func initRepository() error {
	daemonLocked, err := fsrepo.LockedByOtherProcess(ConfigRoot)
	if err != nil {
		return err
	}
	log.Info("checking if daemon is running")
	if daemonLocked {
		log.Debug("ipfs daemon is running")
		return fmt.Errorf("ipfs daemon is running. please stop it to run this command")
	}

	if err := os.MkdirAll(ConfigRoot, 0o770); err != nil {
		return err
	}

	installDatabasePlugins()
	_, err = doInit(io.Discard, ConfigRoot, 2048)
	return err
}

// StartNode starts the IPFS node.
func startNode() (*ipfscore.IpfsNode, coreiface.CoreAPI, error) {
	log.Infow("starting IPFS node", "config", ConfigRoot)
	r, err := fsrepo.Open(ConfigRoot)
	if err != nil {
		return nil, nil, err
	}
	cfg := &ipfscore.BuildCfg{
		Repo:      r,
		Online:    true,
		Permanent: true,
	}

	// We use node.Cancel to stop it instead.
	ctx := context.Background()

	node, err := ipfscore.NewNode(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	node.IsDaemon = true
	node.IsOnline = true

	api, err := ipfsapi.NewCoreAPI(node)
	if err != nil {
		return nil, nil, err
	}
	return node, api, nil
}

// CmdCtx returns a commands.Context for the given node and repo path.
func cmdCtx(node *ipfscore.IpfsNode, repoPath string) commands.Context {
	return commands.Context{
		ConfigRoot: repoPath,
		ConstructNode: func() (*ipfscore.IpfsNode, error) {
			return node, nil
		},
	}
}

func installDatabasePlugins() {
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
	log.Infow("initializing new IPFS repository", "root", repoRoot)
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

	conf.Discovery.MDNS.Enabled = false

	// Prevent from scanning local networks which can trigger netscan alerts.
	// See: https://github.com/ipfs/kubo/issues/7985
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
		return fmt.Errorf("cannot write to %s, incorrect permissions: %w", dir, err)
	}

	return err
}
