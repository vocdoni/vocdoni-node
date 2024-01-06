package ipfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	ihelper "github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	ipfscid "github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/commands"
	"github.com/ipfs/kubo/config"
	ipfscore "github.com/ipfs/kubo/core"
	ipfsapi "github.com/ipfs/kubo/core/coreapi"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
	"github.com/ipfs/kubo/repo/fsrepo/migrations/ipfsfetcher"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"

	"go.vocdoni.io/dvote/log"
)

var ConfigRoot string

// ChunkerTypeSize is the chunker type used by IPFS to calculate to build the DAG.
const ChunkerTypeSize = "size-262144"

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

	if err := os.MkdirAll(ConfigRoot, 0750); err != nil {
		return err
	}

	if err := installDatabasePlugins(); err != nil {
		return err
	}
	_, err = doInit(io.Discard, ConfigRoot, 2048)
	return err
}

// StartNode starts the IPFS node.
func startNode() (*ipfscore.IpfsNode, coreiface.CoreAPI, error) {
	log.Infow("starting IPFS node", "config", ConfigRoot)
	r, err := fsrepo.Open(ConfigRoot)
	if errors.Is(err, fsrepo.ErrNeedMigration) {
		log.Warn("Found outdated ipfs repo, migrations need to be run.")
		r, err = runMigrationsAndOpen(ConfigRoot)
	}
	if err != nil {
		return nil, nil, err
	}
	cfg := &ipfscore.BuildCfg{
		Repo:      r,
		Online:    true,
		Permanent: true,
		Routing: func(args libp2p.RoutingOptionArgs) (routing.Routing, error) {
			args.OptimisticProvide = true
			return libp2p.DHTOption(args)
		},
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

// runMigrationsAndOpen fetches and applies migrations just like upstream kubo does
// and returns fsrepo.Open(ConfigRoot)
func runMigrationsAndOpen(ConfigRoot string) (repo.Repo, error) {
	// Read Migration section of IPFS config
	migrationCfg, err := migrations.ReadMigrationConfig(ConfigRoot, "")
	if err != nil {
		return nil, err
	}

	// Define function to create IPFS fetcher.  Do not supply an
	// already-constructed IPFS fetcher, because this may be expensive and
	// not needed according to migration config. Instead, supply a function
	// to construct the particular IPFS fetcher implementation used here,
	// which is called only if an IPFS fetcher is needed.
	newIpfsFetcher := func(distPath string) migrations.Fetcher {
		return ipfsfetcher.NewIpfsFetcher(distPath, 0, &ConfigRoot, "")
	}

	// Fetch migrations from current distribution, or location from environ
	fetchDistPath := migrations.GetDistPathEnv(migrations.CurrentIpfsDist)

	// Create fetchers according to migrationCfg.DownloadSources
	fetcher, err := migrations.GetMigrationFetcher(migrationCfg.DownloadSources,
		fetchDistPath, newIpfsFetcher)
	if err != nil {
		return nil, err
	}
	defer fetcher.Close()

	if migrationCfg.Keep == "cache" || migrationCfg.Keep == "pin" {
		// Create temp directory to store downloaded migration archives
		migrations.DownloadDirectory, err = os.MkdirTemp("", "migrations")
		if err != nil {
			return nil, err
		}
		// Defer cleanup of download directory so that it gets cleaned up
		// if daemon returns early due to error
		defer func() {
			if migrations.DownloadDirectory != "" {
				_ = os.RemoveAll(migrations.DownloadDirectory)
			}
		}()
	}

	err = migrations.RunMigration(context.TODO(), fetcher, fsrepo.RepoVersion, ConfigRoot, false)
	if err != nil {
		return nil, fmt.Errorf("migrations of ipfs-repo failed: %w", err)
	}

	return fsrepo.Open(ConfigRoot)
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

var installDatabasePlugins = sync.OnceValue(func() error {
	loader, err := loader.NewPluginLoader("")
	if err != nil {
		return err
	}
	if err := loader.Initialize(); err != nil {
		return err
	}
	if err := loader.Inject(); err != nil {
		return err
	}
	return nil
})

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
		if err := fi.Close(); err != nil {
			return err
		}
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesn't exist, check that we can create it
		return os.Mkdir(dir, 0750)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions: %w", dir, err)
	}

	return err
}
