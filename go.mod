module go.vocdoni.io/dvote

go 1.17

// For testing purposes while dvote-protobuf becomes stable
// replace go.vocdoni.io/proto => ../dvote-protobuf

replace github.com/timshannon/badgerhold/v3 => github.com/vocdoni/badgerhold/v3 v3.0.0-20210514115050-2d704df3456f

// Don't upgrade bazil.org/fuse past v0.0.0-20200407214033-5883e5a4b512 for now,
// as it dropped support for GOOS=darwin.
// If you change its version, ensure that "GOOS=darwin go build ./..." still works.

require (
	git.sr.ht/~sircmpwn/go-bare v0.0.0-20210406120253-ab86bc2846d9
	github.com/766b/chi-prometheus v0.0.0-20180509160047-46ac2b31aa30
	github.com/arnaucube/go-blindsecp256k1 v0.0.0-20210323162413-ccaa6313370d
	github.com/cockroachdb/pebble v0.0.0-20211004132338-b2eb88a71826
	github.com/cosmos/iavl v0.15.3
	github.com/deroproject/graviton v0.0.0-20201218180342-ab474f4c94d2
	github.com/dgraph-io/badger/v3 v3.2103.1
	github.com/enriquebris/goconcurrentqueue v0.6.0
	github.com/ethereum/go-ethereum v1.10.8
	github.com/frankban/quicktest v1.13.0
	github.com/glendc/go-external-ip v0.1.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/cors v1.1.1
	github.com/google/go-cmp v0.5.5
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/iden3/go-iden3-crypto v0.0.6-0.20210308142348-8f85683b2cef
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipfs v0.9.1
	github.com/ipfs/go-ipfs-config v0.14.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-keystore v0.0.2
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/klauspost/compress v1.13.6
	github.com/libp2p/go-libp2p v0.14.4
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/libp2p/go-libp2p-discovery v0.5.1
	github.com/libp2p/go-libp2p-kad-dht v0.12.3-0.20210722180723-7706c7bcfdc7
	github.com/libp2p/go-reuseport v0.0.2
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mattn/go-sqlite3 v1.14.9
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/pressly/goose/v3 v3.3.0
	github.com/prometheus/client_golang v1.10.0
	github.com/shirou/gopsutil v3.21.8+incompatible
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/tendermint/tendermint v0.34.13
	github.com/tendermint/tm-db v0.6.4
	github.com/timshannon/badgerhold/v3 v3.0.0-20210415132401-e7c90fb5919f
	github.com/vocdoni/arbo v0.0.0-20211217085703-d56ab859f109
	github.com/vocdoni/go-snark v0.0.0-20210709152824-f6e4c27d7319
	github.com/vocdoni/storage-proofs-eth-go v0.1.6
	go.uber.org/zap v1.18.1
	go.vocdoni.io/proto v1.13.3-0.20211213155005-46b4177904ba
	golang.org/x/crypto v0.0.0-20210920023735-84f357641f63
	golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f
	google.golang.org/protobuf v1.27.1
	nhooyr.io/websocket v1.8.7
)

require github.com/libp2p/go-libp2p-pubsub v0.4.2 // direct

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
