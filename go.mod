module go.vocdoni.io/dvote

go 1.14

// For testing purposes while dvote-protobuf becomes stable
// replace go.vocdoni.io/proto  => ../dvote-protobuf

require (
	bazil.org/fuse v0.0.0-20200524192727-fb710f7dfd05 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.2.0 // indirect
	git.sr.ht/~sircmpwn/go-bare v0.0.0-20201210182351-86af428a8287
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/bren2010/proquint v0.0.0-20201027163346-95122a84635f // indirect
	github.com/cheggaaa/pb v1.0.29 // indirect
	github.com/cosmos/iavl v0.15.3
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/deroproject/graviton v0.0.0-20200906044921-89e9e09f9601
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/ethereum/go-ethereum v1.9.26-0.20201212163632-00d10e610f9f
	github.com/frankban/quicktest v1.11.3
	github.com/gabriel-vasile/mimetype v1.1.2 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/iden3/go-iden3-core v0.0.8-0.20200325104031-1ed04a261b78
	github.com/iden3/go-iden3-crypto v0.0.4
	github.com/ipfs/go-bitswap v0.3.3 // indirect
	github.com/ipfs/go-ds-badger v0.2.6 // indirect
	github.com/ipfs/go-graphsync v0.6.1-0.20210122235421-90b4d163a1bf // indirect
	github.com/ipfs/go-ipfs v0.7.1-0.20210129042248-884a5aebd748
	github.com/ipfs/go-ipfs-blockstore v1.0.3 // indirect
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-pinner v0.1.1 // indirect
	github.com/ipfs/go-ipld-cbor v0.0.5 // indirect
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-path v0.0.9 // indirect
	github.com/ipfs/go-pinning-service-http-client v0.1.0 // indirect
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/ipld/go-car v0.1.1-0.20201119040259-8fe0b741ece2 // indirect
	github.com/ipld/go-ipld-prime-proto v0.1.1 // indirect
	github.com/klauspost/compress v1.11.4
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/libp2p/go-libp2p v0.13.0 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20201026210036-4f868c957324 // indirect
	github.com/libp2p/go-libp2p-core v0.8.0
	github.com/libp2p/go-libp2p-http v0.2.0 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.11.1 // indirect
	github.com/libp2p/go-libp2p-noise v0.1.2 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.4.1 // indirect
	github.com/libp2p/go-libp2p-pubsub-router v0.4.0 // indirect
	github.com/libp2p/go-libp2p-quic-transport v0.10.0 // indirect
	github.com/libp2p/go-netroute v0.1.4 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/miekg/dns v1.1.35 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/polydawn/refmt v0.0.0-20201211092308-30ac6d18308e // indirect
	github.com/prometheus/client_golang v1.9.0
	github.com/recws-org/recws v1.2.2 // indirect
	github.com/shirou/gopsutil v3.20.12+incompatible
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/tendermint/tendermint v0.34.3
	github.com/tendermint/tm-db v0.6.3
	github.com/vocdoni/eth-storage-proof v0.1.4-0.20201128112323-de7513ce5e25
	github.com/vocdoni/multirpc v0.1.9
	github.com/whyrusleeping/cbor-gen v0.0.0-20210118024343-169e9d70c0c2 // indirect
	github.com/whyrusleeping/tar-utils v0.0.0-20201201191210-20a61371de5b // indirect
	gitlab.com/vocdoni/go-dvote v0.6.1-0.20201009163905-60d45cde762f // indirect
	gitlab.com/vocdoni/go-external-ip v0.0.0-20190919225616-59cf485d00da
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	go.vocdoni.io/proto v0.1.8
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	golang.org/x/text v0.3.5 // indirect
	google.golang.org/protobuf v1.25.0
	nhooyr.io/websocket v1.8.6
)

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
