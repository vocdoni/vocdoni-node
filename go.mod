module go.vocdoni.io/dvote

go 1.16

// For testing purposes while dvote-protobuf becomes stable
// replace go.vocdoni.io/proto  => ../dvote-protobuf

require (
	git.sr.ht/~sircmpwn/go-bare v0.0.0-20210227202403-5dae5c48f917
	github.com/arnaucube/go-blindsecp256k1 v0.0.0-20210203222605-876755a714c3
	github.com/cosmos/iavl v0.15.3
	github.com/deroproject/graviton v0.0.0-20200906044921-89e9e09f9601
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/dgraph-io/badger/v3 v3.2011.1
	github.com/ethereum/go-ethereum v1.9.26-0.20201212163632-00d10e610f9f
	github.com/frankban/quicktest v1.11.3
	github.com/google/go-cmp v0.5.4
	github.com/hashicorp/hcl v1.0.1-0.20180906183839-65a6292f0157 // indirect
	github.com/iden3/go-iden3-core v0.0.8-0.20200325104031-1ed04a261b78
	github.com/iden3/go-iden3-crypto v0.0.4
	github.com/ipfs/go-bitswap v0.3.4-0.20210226182239-d1d4afa29da4 // indirect
	github.com/ipfs/go-block-format v0.0.3 // indirect
	github.com/ipfs/go-ipfs v0.7.1-0.20210129042248-884a5aebd748
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-log/v2 v2.1.2 // indirect
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/klauspost/compress v1.11.4
	github.com/libp2p/go-libp2p v0.13.1-0.20210302020805-6a14d8c23942 // indirect
	github.com/libp2p/go-libp2p-autonat v0.4.1 // indirect
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-swarm v0.4.3 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/prometheus/client_golang v1.9.0
	github.com/shirou/gopsutil v3.20.12+incompatible
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/tendermint/tendermint v0.34.8
	github.com/tendermint/tm-db v0.6.4
	github.com/timshannon/badgerhold/v3 v3.0.0-20210415132401-e7c90fb5919f
	github.com/vocdoni/blind-ca v0.1.4
	github.com/vocdoni/multirpc v0.1.21
	github.com/vocdoni/storage-proofs-eth-go v0.1.5
	gitlab.com/vocdoni/go-external-ip v0.0.0-20190919225616-59cf485d00da
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/zap v1.16.0
	go.vocdoni.io/proto v1.0.3-0.20210414183906-e76062846007
	golang.org/x/crypto v0.0.0-20210317152858-513c2a44f670
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sys v0.0.0-20210309074719-68d13333faf2 // indirect
	google.golang.org/protobuf v1.25.0
	nhooyr.io/websocket v1.8.6
)

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
