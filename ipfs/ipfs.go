package ipfs

import (
	"context"
	"fmt"
	"os"
	"path"

	autonat "github.com/libp2p/go-libp2p-autonat-svc"

	ipfsconfig "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	ipfsapi "github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/pkg/errors"

	"gitlab.com/vocdoni/go-dvote/log"
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
		e := "ipfs daemon is running. please stop it to run this command"
		return errors.New(e)
	}

	mode := os.FileMode(int(0770))
	err = os.MkdirAll(ConfigRoot, mode)
	if err != nil {
		return err
	}

	f, err := os.Create(path.Join(ConfigRoot, "version"))
	if err != nil {
		return err
	}

	_, werr := f.Write([]byte(fmt.Sprintf("%d\n", RepoVersion)))
	if werr != nil {
		return werr
	}
	InstallDatabasePlugins()
	_, err = doInit(os.Stdout, ConfigRoot, 2048, []string{}, nil)
	return err
}

func StartNode() (*ipfscore.IpfsNode, coreiface.CoreAPI, error) {
	log.Info("attempting to start IPFS node")
	log.Infof("config root: %s", ConfigRoot)
	r, err := fsrepo.Open(ConfigRoot)
	if err != nil {
		log.Warn("Error opening repo dir")
		return nil, nil, err
	}
	//	defer r.Close()

	ctx := context.Background()

	cfg := &ipfscore.BuildCfg{
		Repo:      r,
		Online:    true,
		Permanent: true,
		/*	ExtraOpts: map[string]bool{
				"mplex":  true,
				"ipnsps": true,
			},
		*/
	}

	node, err := ipfscore.NewNode(ctx, cfg)
	if err != nil {
		log.Warn("error constructing node")
		return nil, nil, err
	}
	node.IsDaemon = true
	node.IsOnline = true

	auts, err := autonat.NewAutoNATService(node.Context(), node.PeerHost)
	if err != nil {
		log.Warn(err)
	}
	node.AutoNAT = auts

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
