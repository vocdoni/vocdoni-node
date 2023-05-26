package ipfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	coreiface "github.com/ipfs/boxo/coreiface"
	ipfscid "github.com/ipfs/go-cid"
	keystore "github.com/ipfs/go-ipfs-keystore"
	files "github.com/ipfs/go-libipfs/files"
	ipfslog "github.com/ipfs/go-log/v2"

	"github.com/ipfs/boxo/coreiface/options"
	corepath "github.com/ipfs/boxo/coreiface/path"
	ipfscmds "github.com/ipfs/kubo/commands"
	ipfscore "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/corehttp"
	"github.com/ipfs/kubo/core/corerepo"
	"github.com/ipfs/kubo/core/coreunix"
	"github.com/ipfs/kubo/repo/fsrepo"
	ipfscrypto "github.com/libp2p/go-libp2p/core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

const (
	// MaxFileSizeBytes is the maximum size of a file to be published to IPFS
	MaxFileSizeBytes = 1024 * 1024 * 100 // 100 MB
	// RetrievedFileCacheSize is the maximum number of files to be cached in memory
	RetrievedFileCacheSize = 128
)

// Handler is the IPFS data storage node handler.
type Handler struct {
	Node          *ipfscore.IpfsNode
	CoreAPI       coreiface.CoreAPI
	DataDir       string
	LogLevel      string
	retrieveCache *lru.Cache

	// cancel helps us stop extra goroutines and listeners which complement
	// the IpfsNode above.
	cancel func()
	maddr  ma.Multiaddr
}

// Init initializes the IPFS node handler and repository.
func (i *Handler) Init(d *types.DataStore) error {
	if i.LogLevel == "" {
		i.LogLevel = "ERROR"
	}
	ipfslog.SetLogLevel("*", i.LogLevel)
	installDatabasePlugins()
	ConfigRoot = d.Datadir
	os.Setenv("IPFS_FD_MAX", "4096")

	// check if needs init
	if !fsrepo.IsInitialized(ConfigRoot) {
		if err := initRepository(); err != nil {
			return err
		}
	}
	node, coreAPI, err := startNode()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	// Start garbage collector, with our cancellable context.
	go func() {
		if err := corerepo.PeriodicGC(ctx, node); err != nil {
			log.Errorw(err, "error running ipfs garbage collector")
		}
	}()
	log.Infow("IPFS initialization",
		"peerID", node.Identity.Pretty(),
		"addresses", node.PeerHost.Addrs(),
		"pubKey", node.PrivateKey.GetPublic(),
	)
	// start http
	cctx := cmdCtx(node, d.Datadir)
	cctx.ReqLog = &ipfscmds.ReqLog{}

	gatewayOpt := corehttp.GatewayOption(corehttp.WebUIPaths...)
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
	i.retrieveCache = lru.New(RetrievedFileCacheSize)

	return nil
}

// Stop stops the IPFS node handler.
func (i *Handler) Stop() error {
	i.cancel()
	return i.Node.Close()
}

// SetMultiAddress sets the multiaddress of the IPFS node.
func (i *Handler) SetMultiAddress(addr string) (err error) {
	i.maddr, err = ma.NewMultiaddr(addr)
	return err
}

// URIprefix returns the URI prefix which identifies the protocol
func (i *Handler) URIprefix() string {
	return "ipfs://"
}

// Publish publishes a file or message to ipfs and returns the resulting CID v1.
func (i *Handler) Publish(ctx context.Context, msg []byte) (cid string, err error) {
	adder, err := coreunix.NewAdder(ctx, i.Node.Pinning, i.Node.Blockstore, i.Node.DAG)
	if err != nil {
		return "", err
	}
	adder.Chunker = ChunkerTypeSize
	adder.CidBuilder = ipfscid.V1Builder{
		Codec:  uint64(multicodec.DagJson),
		MhType: uint64(multihash.SHA2_256),
	}
	msgFile := files.NewBytesFile(msg)
	format, err := adder.AddAllAndPin(ctx, msgFile)
	if err != nil {
		return "", err
	}
	cid = format.Cid().String()
	log.Infow("published file", "protocol", "ipfs", "cid", cid, "size", len(msg))
	return cid, nil
}

// Pin adds a file to ipfs and returns the resulting CID v1.
func (i *Handler) Pin(ctx context.Context, path string) error {
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)
	return i.CoreAPI.Pin().Add(ctx, corepath.New(path))
}

func (i *Handler) addAndPin(ctx context.Context, path string) (corepath.Resolved, error) {
	f, err := unixfsFilesNode(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rpath, err := i.CoreAPI.Unixfs().Add(ctx, f,
		options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(true))
	if err != nil {
		return nil, err
	}

	return rpath, nil
}

// Unpin removes a file pin from ipfs.
func (i *Handler) Unpin(ctx context.Context, path string) error {
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)
	cpath := corepath.New(path)
	if err := cpath.IsValid(); err != nil {
		return fmt.Errorf("invalid path %s: %w", path, err)
	}
	log.Debugf("removing pin %s", cpath.String())
	return i.CoreAPI.Pin().Rm(ctx, cpath, options.Pin.RmRecursive(true))
}

// Stats returns stats about the IPFS node.
func (i *Handler) Stats(ctx context.Context) map[string]interface{} {
	peers, err := i.CoreAPI.Swarm().Peers(ctx)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	addresses, err := i.CoreAPI.Swarm().KnownAddrs(ctx)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	pins, err := i.countPins(ctx)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"peers": len(peers), "addresses": len(addresses), "pins": pins}
}

func (i *Handler) countPins(ctx context.Context) (int, error) {
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

// ListPins returns a map of all pinned CIDs and their types
func (i *Handler) ListPins(ctx context.Context) (map[string]string, error) {
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

// Retrieve gets an IPFS file (either from the p2p network or from the local cache).
// If maxSize is 0, it is set to the hardcoded maximum of MaxFileSizeBytes.
func (i *Handler) Retrieve(ctx context.Context, path string, maxSize int64) ([]byte, error) {
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)

	// check if we have the file in the local cache
	ccontent := i.retrieveCache.Get(path)
	if ccontent != nil {
		log.Debugf("retrieved file %s from cache", path)
		return ccontent.([]byte), nil
	}

	// first resolve the path
	cpath, err := i.CoreAPI.ResolvePath(ctx, corepath.New(path))
	if err != nil {
		return nil, fmt.Errorf("could not resolve path %s", path)
	}

	// then get the file
	f, err := i.CoreAPI.Unixfs().Get(ctx, cpath)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve unixfs file: %w", err)
	}
	file := files.ToFile(f)
	if file == nil {
		return nil, fmt.Errorf("object is not a file")
	}
	defer file.Close()

	fsize, err := file.Size()
	if err != nil {
		return nil, err
	}

	if maxSize == 0 {
		maxSize = MaxFileSizeBytes
	}

	if fsize > int64(maxSize) {
		return nil, fmt.Errorf("file too big: %d", fsize)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("retrieved file is empty")
	}

	if log.Level() >= log.LogLevelDebug {
		toLog := string(content)
		if len(toLog) > 1024 {
			toLog = toLog[:1024] + "..."
		}
		log.Debugf("rawdata received: %s", toLog)
	}

	// Save file to cache for future attempts
	i.retrieveCache.Add(path, content)

	log.Infow("retrieved file", "path", path, "size", fsize)
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
func (i *Handler) PublishIPNSpath(ctx context.Context, path string,
	keyalias string) (coreiface.IpnsEntry, error) {
	rpath, err := i.addAndPin(ctx, path)
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
		rpath,
		options.Name.TTL(time.Minute*10),
		options.Name.Key(keyalias),
	)
}

// AddKeyToKeystore adds a marshaled IPFS private key to the IPFS keystore.
// The key is identified by a unique alias name which can be used for referncing
// that key when using some other IPFS methods.
// Compatible Keys can be generated with NewIPFSkey() function.
func (i *Handler) AddKeyToKeystore(keyalias string, privkey []byte) error {
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

// unixfsFilesNode returns a go-ipfs files.Node given a unix path
func unixfsFilesNode(path string) (files.Node, error) {
	stat, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	f, err := files.NewSerialFile(path, false, stat)
	if err != nil {
		return nil, err
	}
	return f, nil
}
