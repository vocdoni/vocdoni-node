module go.vocdoni.io/dvote

go 1.14

// For testing purposes while dvote-protobuf becomes stable
// replace go.vocdoni.io/proto  => ../dvote-protobuf

require (
	git.sr.ht/~sircmpwn/go-bare v0.0.0-20201210182351-86af428a8287
	github.com/arnaucube/go-blindsecp256k1 v0.0.0-20210203222605-876755a714c3
	github.com/cosmos/iavl v0.15.3
	github.com/deroproject/graviton v0.0.0-20200906044921-89e9e09f9601
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/ethereum/go-ethereum v1.9.26-0.20201212163632-00d10e610f9f
	github.com/frankban/quicktest v1.11.3
	github.com/google/go-cmp v0.5.4
	github.com/iden3/go-iden3-core v0.0.8-0.20200325104031-1ed04a261b78
	github.com/iden3/go-iden3-crypto v0.0.4
	github.com/ipfs/go-ipfs v0.7.1-0.20210129042248-884a5aebd748
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/klauspost/compress v1.11.4
	github.com/libp2p/go-libp2p-core v0.8.0
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/nsqio/go-diskqueue v1.0.0
	github.com/prometheus/client_golang v1.9.0
	github.com/shirou/gopsutil v3.20.12+incompatible
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/tendermint/tendermint v0.34.3
	github.com/tendermint/tm-db v0.6.3
	github.com/vocdoni/blind-ca v0.1.4
	github.com/vocdoni/multirpc v0.1.12
	github.com/vocdoni/storage-proofs-eth-go v0.1.5
	gitlab.com/vocdoni/go-external-ip v0.0.0-20190919225616-59cf485d00da
	go.uber.org/zap v1.16.0
	go.vocdoni.io/proto v0.1.8
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	google.golang.org/protobuf v1.25.0
	nhooyr.io/websocket v1.8.6
)

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
