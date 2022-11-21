package data

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ipfscid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	keystore "github.com/ipfs/go-ipfs-keystore"
	ipfslog "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	ipfscmds "github.com/ipfs/kubo/commands"
	ipfscore "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/corehttp"
	"github.com/ipfs/kubo/core/corerepo"
	"github.com/ipfs/kubo/repo/fsrepo"
	ipfscrypto "github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multicodec"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/ipfs"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

const (
	MaxFileSizeBytes       = 1024 * 1024 * 50 // 50 MB
	RetrievedFileCacheSize = 128
)

type IPFSHandle struct {
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
	i.retrieveCache = lru.New(RetrievedFileCacheSize)
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

// Publish publishes a message to ipfs and returns the resulting CID v1.
func (i *IPFSHandle) Publish(ctx context.Context, msg []byte) (cid string, err error) {
	// needs options.Unixfs.CidVersion(1) since CID v0 calculation is broken (differs from ipfscid.Sum)
	rpath, err := i.CoreAPI.Unixfs().Add(ctx, files.NewBytesFile(msg), options.Unixfs.CidVersion(1))
	if err != nil {
		return "", fmt.Errorf("could not publish: %s", err)
	}
	// return the CIDv1 hash only computed by our ipfs halper function (without the /ipfs/ prefix)
	cid = IPFSCIDv1json(rpath.Cid()).String()
	log.Infof("published file: %s", cid)
	return cid, nil
}

// AddFile adds a file to ipfs and returns the resulting CID v1.
func (i *IPFSHandle) AddAndPin(ctx context.Context, path string) (cid string, err error) {
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)
	rpath, err := i.addAndPin(ctx, path)
	if err != nil {
		return "", err
	}
	return rpath.Root().String(), nil
}

func (i *IPFSHandle) addAndPin(ctx context.Context, path string) (corepath.Resolved, error) {
	f, err := unixfsFilesNode(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// needs options.Unixfs.CidVersion(1) since CID v0 calculation is broken (differs from ipfscid.Sum)
	rpath, err := i.CoreAPI.Unixfs().Add(ctx, f, options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(true))
	if err != nil {
		return nil, err
	}

	return rpath, nil
}

func (i *IPFSHandle) Pin(ctx context.Context, path string) error {
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)
	cpath := corepath.New(path)
	log.Infof("adding pin %s", cpath.String())
	return i.CoreAPI.Pin().Add(ctx, cpath, options.Pin.Recursive(true))
}

func (i *IPFSHandle) Unpin(ctx context.Context, path string) error {
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)
	cpath := corepath.New(path)
	log.Infof("removing pin %s", cpath.String())
	return i.CoreAPI.Pin().Rm(ctx, cpath, options.Pin.RmRecursive(true))
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
	path = strings.Replace(path, "ipfs://", "/ipfs/", 1)

	// check if we have the file in the local cache
	ccontent := i.retrieveCache.Get(path)
	if ccontent != nil {
		log.Debugf("retrieved file %s from cache", path)
		return ccontent.([]byte), nil
	}

	cnode, err := i.CoreAPI.ResolveNode(ctx, corepath.New(path))
	if err != nil {
		return nil, fmt.Errorf("could not resolve node: %w", err)
	}
	log.Debugf("rawdata received: %s", cnode.RawData())
	if maxSize == 0 {
		maxSize = MaxFileSizeBytes
	}
	if s, err := cnode.Size(); s > uint64(maxSize) || err != nil {
		return nil, fmt.Errorf("file too big or size cannot be obtained: (size:%d)", s)
	}
	content := cnode.RawData()
	if len(content) == 0 {
		return nil, fmt.Errorf("retrieved file is empty")
	}

	// Save file to cache for future attempts
	i.retrieveCache.Add(path, content)

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

// CalculateIPFSCIDv1json calculates the IPFS Cid hash (v1) from a bytes buffer,
// using parameters Codec: JSON, MhType: SHA2_256
func CalculateIPFSCIDv1json(data []byte) (cid string) {
	format := ipfscid.V1Builder{
		MhType: uint64(multicodec.Sha2_256),
	}
	c, err := format.Sum(data)
	if err != nil {
		log.Warnf("%v", err)
		return ""
	}
	log.Debugf("computed cid: %s", IPFSCIDv1json(c).String())
	return IPFSCIDv1json(c).String()
}

// IPFSCIDv1json converts any given Cid (v0 or v1) into a v1 with Codec: JSON (0x0200)
func IPFSCIDv1json(cid ipfscid.Cid) ipfscid.Cid {
	// The multicodec indicates the format of the target content
	// it helps people and software to know how to interpret that
	// content after the content is fetched
	return ipfscid.NewCidV1(uint64(multicodec.Json), cid.Hash())
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
