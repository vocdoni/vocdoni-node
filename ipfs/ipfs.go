package ipfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ipfsconfig "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	ipfsapi "github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	coreiface "github.com/ipfs/interface-go-ipfs-core"

	"go.vocdoni.io/dvote/log"
)

var ConfigRoot string

const RepoVersion = 7

func Init() error {
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

	if err := os.WriteFile(filepath.Join(ConfigRoot, "version"),
		[]byte(fmt.Sprintf("%d\n", RepoVersion)), 0o666,
	); err != nil {
		return err
	}

	InstallDatabasePlugins()
	_, err = doInit(io.Discard, ConfigRoot, 2048)
	return err
}

func StartNode() (*ipfscore.IpfsNode, coreiface.CoreAPI, error) {
	log.Info("attempting to start IPFS node")
	log.Infof("config root: %s", ConfigRoot)
	r, err := fsrepo.Open(ConfigRoot)
	if err != nil {
		log.Warn("error opening repo dir")
		return nil, nil, err
	}
	cfg := &ipfscore.BuildCfg{
		Repo:      r,
		Online:    true,
		Permanent: true,

		// ExtraOpts: map[string]bool{
		// 	"mplex":  true,
		// 	"ipnsps": true,
		// },
	}

	// We use node.Cancel to stop it instead.
	ctx := context.Background()

	node, err := ipfscore.NewNode(ctx, cfg)
	if err != nil {
		log.Warn("error constructing node")
		return nil, nil, err
	}
	node.IsDaemon = true
	node.IsOnline = true

	api, err := ipfsapi.NewCoreAPI(node)
	if err != nil {
		log.Warn("error constructing core API")
		return nil, nil, err
	}

	return node, api, nil
}

func CmdCtx(node *ipfscore.IpfsNode, repoPath string) commands.Context {
	return commands.Context{
		ConfigRoot: repoPath,
		LoadConfig: func(path string) (*ipfsconfig.Config, error) {
			return node.Repo.Config()
		},
		ConstructNode: func() (*ipfscore.IpfsNode, error) {
			return node, nil
		},
	}
}
