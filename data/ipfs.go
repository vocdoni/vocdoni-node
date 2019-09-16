package data

import (
	"context"
	"errors"
	"io/ioutil"
	"os/user"

	"os"

	//"os/exec"
	"strings"

	files "github.com/ipfs/go-ipfs-files"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	crypto "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/ipfs"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type IPFSHandle struct {
	Node    *ipfscore.IpfsNode
	CoreAPI coreiface.CoreAPI
	DataDir string
}

// check if ipfs base dir exists
func checkIPFSDirExists(path string) (bool, error) {
	usr, err := user.Current()
	if err != nil {
		return false, errors.New("Cannot get $HOME")
	}
	userHomeDir := usr.HomeDir
	_, err = os.Stat(userHomeDir + "/." + path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (i *IPFSHandle) Init(d *types.DataStore) error {
	ipfs.InstallDatabasePlugins()
	ipfs.ConfigRoot = d.Datadir
	//check if needs init
	if !fsrepo.IsInitialized(ipfs.ConfigRoot) {
		err := ipfs.Init()
		if err != nil {
			log.Errorf("Error in IPFS init: %s", err)
		}
	}
	nd, coreAPI, err := ipfs.StartNode()
	if err != nil {
		return err
	}
	log.Infof("IPFS Peer ID: %s", nd.Identity.Pretty())

	i.Node = nd
	i.CoreAPI = coreAPI
	i.DataDir = d.Datadir

	return nil
}

//GetURIprefix returns the URI prefix which identifies the protocol
func (i *IPFSHandle) GetURIprefix() string {
	return "ipfs://"
}

//PublishFile publishes a file specified by root to ipfs
func PublishFile(root []byte, nd *ipfscore.IpfsNode) (string, error) {
	rootHash, err := addAndPin(nd, string(root))
	if err != nil {
		return "", err
	}
	return rootHash, nil
}

//PublishBytes publishes a file containing msg to ipfs
func PublishBytes(msg []byte, fileDir string, nd *ipfscore.IpfsNode) (string, error) {
	filePath := fileDir + "/" + string(crypto.HashRaw(string(msg))) + ".txt"
	log.Infof("Publishing file: %s", filePath)
	err := ioutil.WriteFile(filePath, msg, 0666)
	rootHash, err := addAndPin(nd, filePath)
	if err != nil {
		return "", err
	}
	return rootHash, nil
}

//Publish publishes a message to ipfs
func (i *IPFSHandle) Publish(msg []byte) (string, error) {
	//if sent a message instead of a file
	roothash, err := PublishBytes(msg, i.DataDir, i.Node)
	return roothash, err
}

func addAndPin(n *ipfscore.IpfsNode, root string) (rootHash string, err error) {
	defer n.Blockstore.PinLock().Unlock()
	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, false, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fileAdder, err := coreunix.NewAdder(context.Background(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	node, err := fileAdder.AddAllAndPin(f)
	if err != nil {
		return "", err
	}
	return node.Cid().String(), nil
}

func (i *IPFSHandle) Pin(path string) error {
	p := corepath.New(path)
	rp, err := i.CoreAPI.ResolvePath(context.Background(), p)
	if err != nil {
		return err
	}
	return i.CoreAPI.Pin().Add(context.Background(), rp, options.Pin.Recursive(true))
}

func (i *IPFSHandle) Unpin(path string) error {
	p := corepath.New(path)
	rp, err := i.CoreAPI.ResolvePath(context.Background(), p)
	if err != nil {
		return err
	}
	return i.CoreAPI.Pin().Rm(context.Background(), rp, options.Pin.RmRecursive(true))
}

func (i *IPFSHandle) ListPins() (map[string]string, error) {
	pins, err := i.CoreAPI.Pin().Ls(context.Background())
	if err != nil {
		return nil, err
	}
	var pinMap map[string]string
	pinMap = make(map[string]string)
	for _, p := range pins {
		pinMap[p.Path().String()] = p.Type()
	}
	return pinMap, nil
}

func (i *IPFSHandle) Retrieve(path string) ([]byte, error) {
	ctx := context.Background()

	if !strings.HasPrefix(path, "/ipfs/") {
		path = "/ipfs/" + path
	}

	pth := corepath.New(path)

	nd, err := i.CoreAPI.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}

	r, ok := nd.(files.File)
	if !ok {
		return nil, errors.New("Received incorrect type from Unixfs().Get()")
	}

	return ioutil.ReadAll(r)
}
