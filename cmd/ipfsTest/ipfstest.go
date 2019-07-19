package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	ipfscmds "github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/ipfs"

	"net/http"
	_ "net/http/pprof"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core/coreunix"
	flag "github.com/spf13/pflag"
)

const programName = `dvote-ipfs-test`

type IPFSHandle struct {
	nd      *ipfscore.IpfsNode
	coreAPI coreiface.CoreAPI
}

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal("No user")
	}
	defaultPath := usr.HomeDir + "/.ipfstest"
	path := flag.String("ipfspath", defaultPath, "ipfspath. Specify filepath for ipfs config file")
	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	ipfs.InstallDatabasePlugins()
	ipfs.ConfigRoot = *path

	if !fsrepo.IsInitialized(ipfs.ConfigRoot) {
		err := ipfs.Init()
		if err != nil {
			log.Println("Error in init: ")
			log.Println(err)
		}
	}
	nd, coreAPI, err := ipfs.StartNode()
	if err != nil {
		log.Printf("Error in StartNode: %s ", err.Error())
	}
	log.Println("Peer ID: ", nd.Identity.Pretty())

	//start http
	cctx := ipfs.CmdCtx(nd, defaultPath)
	cctx.ReqLog = &ipfscmds.ReqLog{}

	gatewayOpt := corehttp.GatewayOption(true, corehttp.WebUIPaths...)
	var opts = []corehttp.ServeOption{
		//		corehttp.MetricsCollectionOption("api"),
		//		corehttp.CheckVersionOption(),
		corehttp.CommandsOption(cctx),
		corehttp.WebUIOption,
		gatewayOpt,
		//		corehttp.VersionOption(),
		//		defaultMux("/debug/vars"),
		//		defaultMux("/debug/pprof/"),
		//		corehttp.MutexFractionOption("/debug/pprof-mutex/"),
		//		corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
		//		corehttp.LogOption(),
	}
	go corehttp.ListenAndServe(nd, "/ip4/0.0.0.0/tcp/5001", opts...)

	var ipfsHandler IPFSHandle
	ipfsHandler.nd = nd
	ipfsHandler.coreAPI = coreAPI

	ipfs.ProgramName = programName

	clusterPath := *path + "/.cluster"
	clusterConfig := new(config.ClusterCfg)
	err = ipfs.InitCluster(clusterPath, "conf.json", "id.json", *clusterConfig)
	if err != nil {
		log.Fatalf("Error initializing ipfs cluster: %v", err)
	}
	go ipfs.RunCluster(*clusterConfig)

	ticker := time.NewTicker(time.Second * time.Duration(10))
	signalChan := make(chan os.Signal, 10)
	signal.Notify(
		signalChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGKILL,
	)

	for {
		select {
		case <-ticker.C:
			cid, err := ipfsHandler.Publish(*path + "/0001.txt")
			if err != nil {
				log.Printf("Publishing error: %v", err)
			} else {
				log.Printf("CID:%v", cid)
			}

			err = ipfsHandler.Pin(cid)
			if err != nil {
				log.Printf("Pinning error: %v", err)
			} else {
				log.Printf("Path pinned")
			}

			err = ipfsHandler.UnPin(cid)
			if err != nil {
				log.Printf("Unpinning error: %v", err)
			} else {
				log.Printf("Path unpinned")
			}

			time.Sleep(3 * time.Second)

			pinlist, err := ipfsHandler.ListPins()
			if err != nil {
				log.Printf("Pin list error: %v", err)
			} else {
				log.Println("Pin list:")
				for key, value := range pinlist {
					log.Printf("\tKey: %v", key)
					log.Printf("\tValue: %v", value)
				}
			}

			data, err := ipfsHandler.Retrieve(cid)
			if err != nil {
				log.Printf("Retrieve error: %v", err)
			} else {
				log.Printf("Retrieved message: %s", data)
			}

		case <-signalChan:
			nd.Close()
			os.Exit(1)
		default:
			continue
		}
	}
}

func (i *IPFSHandle) Publish(root string) (string, error) {
	rootHash, err := addAndPin(i.nd, root)
	if err != nil {
		return "", err
	}
	return rootHash, nil
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
	rp, err := i.coreAPI.ResolvePath(context.Background(), p)
	if err != nil {
		return err
	}
	return i.coreAPI.Pin().Add(context.Background(), rp, options.Pin.Recursive(true))
}

func (i *IPFSHandle) UnPin(path string) error {
	p := corepath.New(path)
	rp, err := i.coreAPI.ResolvePath(context.Background(), p)
	if err != nil {
		return err
	}
	return i.coreAPI.Pin().Rm(context.Background(), rp, options.Pin.RmRecursive(true))
}

func (i *IPFSHandle) ListPins() (map[string]string, error) {
	pins, err := i.coreAPI.Pin().Ls(context.Background())
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

	nd, err := i.coreAPI.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}

	r, ok := nd.(files.File)
	if !ok {
		return nil, errors.New("Received incorrect type from Unixfs().Get()")
	}

	return ioutil.ReadAll(r)
}
