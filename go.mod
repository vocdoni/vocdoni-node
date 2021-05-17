module go.vocdoni.io/dvote

go 1.16

// For testing purposes while dvote-protobuf becomes stable
// replace go.vocdoni.io/proto  => ../dvote-protobuf

replace github.com/timshannon/badgerhold/v3 => github.com/vocdoni/badgerhold/v3 v3.0.0-20210514115050-2d704df3456f

// Don't upgrade bazil.org/fuse past v0.0.0-20200407214033-5883e5a4b512 for now,
// as it dropped support for GOOS=darwin.
// If you change its version, ensure that "GOOS=darwin go build ./..." still works.

require (
	bazil.org/fuse v0.0.0-20200407214033-5883e5a4b512 // indirect
	git.sr.ht/~sircmpwn/go-bare v0.0.0-20210227202403-5dae5c48f917
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/arnaucube/go-blindsecp256k1 v0.0.0-20210203222605-876755a714c3
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/bren2010/proquint v0.0.0-20201027163346-95122a84635f // indirect
	github.com/cosmos/iavl v0.15.3
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/deroproject/graviton v0.0.0-20200906044921-89e9e09f9601
	github.com/dgraph-io/badger/v3 v3.2011.1
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/ethereum/go-ethereum v1.9.26-0.20201212163632-00d10e610f9f
	github.com/frankban/quicktest v1.13.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/cors v1.1.1
	github.com/google/go-cmp v0.5.5
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hashicorp/hcl v1.0.1-0.20180906183839-65a6292f0157 // indirect
	github.com/iden3/go-iden3-core v0.0.8-0.20200325104031-1ed04a261b78
	github.com/iden3/go-iden3-crypto v0.0.4
	github.com/ipfs/go-bitswap v0.3.4-0.20210226182239-d1d4afa29da4 // indirect
	github.com/ipfs/go-block-format v0.0.3 // indirect
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-filestore v1.0.0 // indirect
	github.com/ipfs/go-graphsync v0.6.1-0.20210122235421-90b4d163a1bf // indirect
	github.com/ipfs/go-ipfs v0.7.1-0.20210129042248-884a5aebd748
	github.com/ipfs/go-ipfs-blockstore v1.0.3 // indirect
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-log/v2 v2.1.2 // indirect
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/ipld/go-car v0.1.1-0.20201119040259-8fe0b741ece2 // indirect
	github.com/klauspost/compress v1.11.4
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/libp2p/go-libp2p v0.13.1-0.20210302020805-6a14d8c23942
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20201026210036-4f868c957324 // indirect
	github.com/libp2p/go-libp2p-autonat v0.4.1 // indirect
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-swarm v0.4.3 // indirect
	github.com/libp2p/go-netroute v0.1.4 // indirect
	github.com/libp2p/go-reuseport v0.0.2
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/miekg/dns v1.1.35 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/p4u/recws v1.2.2-0.20201005083112-7be7f9397e75
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/polydawn/refmt v0.0.0-20201211092308-30ac6d18308e // indirect
	github.com/prometheus/client_golang v1.9.0
	github.com/shirou/gopsutil v3.20.12+incompatible
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/tendermint/tendermint v0.34.10
	github.com/tendermint/tm-db v0.6.4
	github.com/timshannon/badgerhold/v3 v3.0.0-20210415132401-e7c90fb5919f
	github.com/vocdoni/storage-proofs-eth-go v0.1.5
	github.com/whyrusleeping/cbor-gen v0.0.0-20210118024343-169e9d70c0c2 // indirect
	gitlab.com/vocdoni/go-external-ip v0.0.0-20190919225616-59cf485d00da
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	go.vocdoni.io/proto v1.0.3
	golang.org/x/crypto v0.0.0-20210317152858-513c2a44f670
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sys v0.0.0-20210309074719-68d13333faf2 // indirect
	golang.org/x/text v0.3.5 // indirect
	google.golang.org/protobuf v1.25.0
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
	nhooyr.io/websocket v1.8.6
)

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
