package ipfs

import (
	"context"
	"os"
	"fmt"
	"path"

	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/commands"
	ipfsconfig "github.com/ipfs/go-ipfs-config"
	ipfsapi "github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	errors "github.com/pkg/errors"
	"gitlab.com/vocdoni/go-dvote/log"
)

var ConfigRoot string

const RepoVersion = 7

func Init() error {
	daemonLocked, err := fsrepo.LockedByOtherProcess(ConfigRoot)
	if err != nil {
		return err
	}

	log.Infof("checking if daemon is running...")
	if daemonLocked {
		log.Debugf("ipfs daemon is running")
		e := "ipfs daemon is running. please stop it to run this command"
		return errors.New(e)
	}

	f, err := os.Create(path.Join(ConfigRoot, "version"))
	if err != nil {
		return err
	}
	_, werr := f.Write([]byte(fmt.Sprintf("%d\n", RepoVersion)))
	if werr != nil {
		return werr
	}

	var profiles []string
	InstallDatabasePlugins()
	return doInit(os.Stdout, ConfigRoot, 2048, profiles, nil)
}

func StartNode() (*ipfscore.IpfsNode, coreiface.CoreAPI, error) {
	log.Infof("Attempting to start node...")
	r, err := fsrepo.Open(ConfigRoot)
	if err != nil {
		log.Infof("Error opening repo dir")
		return nil, nil, err
	}
	//	defer r.Close()

	ctx := context.Background()

	cfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
		/*
			ExtraOpts: map[string]bool{
				"mplex":  true,
				"ipnsps": true,
			},
			*/
	}

	node, err := ipfscore.NewNode(ctx, cfg)
	if err != nil {
		log.Infof("Error constructing node")
		return nil, nil, err
	}
	node.IsDaemon = true
	node.IsOnline = true

	api, err := ipfsapi.NewCoreAPI(node)
	if err != nil {
		log.Info("Error constructing core API")
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