package data

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	keystore "github.com/ipfs/go-ipfs-keystore"
	ipfscmds "github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/core/corerepo"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	ipfslog "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	ipfscrypto "github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	crypto "go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/ipfs"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

const (
	MaxFileSizeBytes      = 1024 * 1024 * 50 // 50 MB
	RetrivedFileCacheSize = 128
)

type IPFSHandle struct {
	Node         *ipfscore.IpfsNode
	CoreAPI      coreiface.CoreAPI
	DataDir      string
	LogLevel     string
	retriveCache *lru.Cache

	// cancel helps us stop extra goroutines and listeners which complement
	// the IpfsNode above.
	cancel func()
	maddr  ma.Multiaddr
}

func (i *IPFSHandle) Init(d *types.DataStore) error {
	if i.LogLevel == "" {
		i.LogLevel = "ERROR"
	}
	ipfslog.SetLogLevel("*", i.LogLevel)
	ipfs.InstallDatabasePlugins()
	ipfs.ConfigRoot = d.Datadir

	os.Setenv("IPFS_FD_MAX", "1024")

	// check if needs init
	if !fsrepo.IsInitialized(ipfs.ConfigRoot) {
		if err := ipfs.Init(); err != nil {
			log.Errorf("error in IPFS init: %s", err)
		}
	}
	node, coreAPI, err := ipfs.StartNode()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	// Start garbage collector, with our cancellable context.
	go corerepo.PeriodicGC(ctx, node)

	log.Infof("IPFS peerID: %s", node.Identity.Pretty())
	// start http
	cctx := ipfs.CmdCtx(node, d.Datadir)
	cctx.ReqLog = &ipfscmds.ReqLog{}

	gatewayOpt := corehttp.GatewayOption(true, corehttp.WebUIPaths...)
	opts := []corehttp.ServeOption{
		corehttp.CommandsOption(cctx),
		corehttp.WebUIOption,
		gatewayOpt,
	}

	if i.maddr == nil {
		if err := i.SetMultiAddress("/ip4/0.0.0.0/tcp/5001"); err != nil {
			return err
		}
	}

	list, err := manet.Listen(i.maddr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		list.Close()
	}()
	// The address might have changed, if the port was 0; use list.Multiaddr
	// to fetch the final one.

	// Avoid corehttp.ListenAndServe, since it doesn't provide the final
	// address, and always prints to stdout.
	go corehttp.Serve(node, manet.NetListener(list), opts...)

	i.Node = node
	i.CoreAPI = coreAPI
	i.DataDir = d.Datadir
	i.retriveCache = lru.New(RetrivedFileCacheSize)
	return nil
}

func (i *IPFSHandle) Stop() error {
	i.cancel()
	return i.Node.Close()
}

func (i *IPFSHandle) SetMultiAddress(addr string) (err error) {
	i.maddr, err = ma.NewMultiaddr(addr)
	return err
}

// URIprefix returns the URI prefix which identifies the protocol
func (i *IPFSHandle) URIprefix() string {
	return "ipfs://"
}

// PublishBytes publishes a file containing msg to ipfs
func (i *IPFSHandle) publishBytes(ctx context.Context, msg []byte, fileDir string) (string, error) {
	filePath := fmt.Sprintf("%s/%x", fileDir, crypto.HashRaw(msg))
	log.Infof("publishing file: %s", filePath)
	err := os.WriteFile(filePath, msg, 0o666)
	if err != nil {
		return "", err
	}
	rootHash, err := i.AddAndPin(ctx, filePath)
	if err != nil {
		return "", err
	}
	return rootHash, nil
}

// Publish publishes a message to ipfs
func (i *IPFSHandle) Publish(ctx context.Context, msg []byte) (string, error) {
	// if sent a message instead of a file
	return i.publishBytes(ctx, msg, i.DataDir)
}

func (i *IPFSHandle) AddAndPin(ctx context.Context, root string) (rootHash string, err error) {

	defer i.Node.Blockstore.PinLock().Unlock()
	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, false, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fileAdder, err := coreunix.NewAdder(ctx, i.Node.Pinning, i.Node.Blockstore, i.Node.DAG)
	if err != nil {
		return "", err
	}

	node, err := fileAdder.AddAllAndPin(f)
	if err != nil {
		return "", err
	}
	return node.Cid().String(), nil
}

func (i *IPFSHandle) Pin(ctx context.Context, path string) error {
	// path = strings.ReplaceAll(path, "/ipld/", "/ipfs/")

	p := corepath.New(path)
	rp, err := i.CoreAPI.ResolvePath(ctx, p)
	if err != nil {
		return err
	}
	return i.CoreAPI.Pin().Add(ctx, rp, options.Pin.Recursive(true))
}

func (i *IPFSHandle) Unpin(ctx context.Context, path string) error {
	p := corepath.New(path)
	rp, err := i.CoreAPI.ResolvePath(ctx, p)
	if err != nil {
		return err
	}
	return i.CoreAPI.Pin().Rm(ctx, rp, options.Pin.RmRecursive(true))
}

func (i *IPFSHandle) Stats(ctx context.Context) (string, error) {
	peers, err := i.CoreAPI.Swarm().Peers(ctx)
	if err != nil {
		return "", err
	}
	addresses, err := i.CoreAPI.Swarm().KnownAddrs(ctx)
	if err != nil {
		return "", err
	}
	pins, err := i.countPins(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("peers:%d addresses:%d pins:%d", len(peers), len(addresses), pins), nil
}

func (i *IPFSHandle) countPins(ctx context.Context) (int, error) {
	// Note that pins is a channel that gets closed when finished.
	// We MUST range over the entire channel to not leak goroutines.
	// Maybe there is a way to get the total number of pins without
	// iterating over them?
	pins, err := i.CoreAPI.Pin().Ls(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for pin := range pins {
		if err := pin.Err(); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func (i *IPFSHandle) ListPins(ctx context.Context) (map[string]string, error) {
	// Note that pins is a channel that gets closed when finished.
	// We MUST range over the entire channel to not leak goroutines.
	pins, err := i.CoreAPI.Pin().Ls(ctx)
	if err != nil {
		return nil, err
	}
	pinMap := make(map[string]string)
	for pin := range pins {
		if err := pin.Err(); err != nil {
			return nil, err
		}
		pinMap[pin.Path().String()] = pin.Type()
	}
	return pinMap, nil
}

// Retrieve gets an IPFS file (either from the p2p network or from the local cache)
func (i *IPFSHandle) Retrieve(ctx context.Context, path string, maxSize int64) ([]byte, error) {
	path = strings.TrimPrefix(path, "ipfs://")
	ccontent := i.retriveCache.Get(path)
	if ccontent != nil {
		log.Debugf("retreived file %s from cache", path)
		return ccontent.([]byte), nil
	}
	rpath, err := i.CoreAPI.ResolvePath(ctx, corepath.New(path))
	if err != nil {
		return nil, fmt.Errorf("resolvepath: %w", err)
	}
	if err := rpath.IsValid(); err != nil {
		return nil, fmt.Errorf("ipfs path is invalid")
	}
	node, err := i.CoreAPI.Unixfs().Get(ctx, rpath)
	if err != nil {
		return nil, err
	}
	defer node.Close()
	if maxSize == 0 {
		maxSize = MaxFileSizeBytes
	}
	if s, err := node.Size(); s > maxSize || err != nil {
		return nil, fmt.Errorf("file too big or size cannot be obtained")
	}
	r, ok := node.(files.File)
	if !ok {
		return nil, fmt.Errorf("received incorrect type from Unixfs().Get()")
	}

	// Fetch the file content
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("retreived file is empty")
	}
	// Save file to cache for future attempts
	i.retriveCache.Add(path, content)

	return content, nil
}

// PublishIPNSpath creates or updates an IPNS record with the content of a
// filesystem path (a single file or a directory).
//
// The IPNS record is published under the scope of the private key identified
// by the keyalias parameter. New keys can be created using method AddKeyToKeystore
// and function NewIPFSkey() both available on this package.
//
// The execution of this method might take a while (some minutes),
// so the caller must handle properly the logic by using goroutines, channels or other
// mechanisms in order to not block the whole program execution.
func (i *IPFSHandle) PublishIPNSpath(ctx context.Context, path string,
	keyalias string) (coreiface.IpnsEntry, error) {
	root, err := i.AddAndPin(ctx, path)
	if err != nil {
		return nil, err
	}
	c, err := cid.Parse(root)
	if err != nil {
		return nil, err
	}
	if keyalias == "" {
		ck, err := i.CoreAPI.Key().Self(ctx)
		if err != nil {
			return nil, err
		}
		keyalias = ck.Name()
	}
	return i.CoreAPI.Name().Publish(
		ctx,
		corepath.IpfsPath(c),
		options.Name.TTL(time.Minute*10),
		options.Name.Key(keyalias),
	)
}

// AddKeyToKeystore adds a marshaled IPFS private key to the IPFS keystore.
// The key is identified by a unique alias name which can be used for referncing
// that key when using some other IPFS methods.
// Compatible Keys can be generated with NewIPFSkey() function.
func (i *IPFSHandle) AddKeyToKeystore(keyalias string, privkey []byte) error {
	pk, err := ipfscrypto.UnmarshalPrivateKey(privkey)
	if err != nil {
		return err
	}
	_ = i.Node.Repo.Keystore().Delete(keyalias)
	if err := i.Node.Repo.Keystore().Put(keyalias, pk); err != nil {
		if err != keystore.ErrKeyExists {
			return err
		}
	}
	return nil
}

// NewIPFSkey generates a new IPFS private key (ECDSA/256bit) and returns its
// marshaled bytes representation.
func NewIPFSkey() []byte {
	// These functions must not return error since all input parameters
	// are predefined, so we panic if an error returned.
	privKey, _, err := ipfscrypto.GenerateKeyPair(ipfscrypto.ECDSA, 256)
	if err != nil {
		panic(err)
	}
	encPrivKey, err := ipfscrypto.MarshalPrivateKey(privKey)
	if err != nil {
		panic(err)
	}
	return encPrivKey
}
