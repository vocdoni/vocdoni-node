module gitlab.com/vocdoni/go-dvote

go 1.14

require (
	github.com/ChainSafe/go-schnorrkel v0.0.0-20200115165343-aa45d48b5ed6 // indirect
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/VictoriaMetrics/fastcache v1.5.7 // indirect
	github.com/aristanetworks/goarista v0.0.0-20200423211322-0b5ff220aee9 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20190912175916-7055855a373f // indirect
	github.com/decred/dcrd/dcrec/secp256k1 v1.0.2
	github.com/dgraph-io/badger/v2 v2.0.1-rc1.0.20200409094109-809725940698
	github.com/dgraph-io/ristretto v0.0.2 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/elastic/gosigar v0.10.5 // indirect
	github.com/ethereum/go-ethereum v1.9.13
	github.com/gballet/go-libpcsclite v0.0.0-20191108122812-4678299bea08 // indirect
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/go-chi/cors v1.0.0
	github.com/golang/protobuf v1.3.5 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/gopherjs/gopherjs v0.0.0-20190812055157-5d271430af9f // indirect
	github.com/gorilla/websocket v1.4.1
	github.com/gtank/merlin v0.1.1 // indirect
	github.com/iden3/go-iden3-core v0.0.8-0.20200325104031-1ed04a261b78
	github.com/iden3/go-iden3-crypto v0.0.4
	github.com/ipfs/go-filestore v1.0.0 // indirect
	github.com/ipfs/go-ipfs v0.4.22-0.20200313001058-457b6e79ff3c
	github.com/ipfs/go-ipfs-config v0.2.1
	github.com/ipfs/go-ipfs-files v0.0.6
	github.com/ipfs/go-log v1.0.2
	github.com/ipfs/interface-go-ipfs-core v0.2.6
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/karalabe/usb v0.0.0-20191104083709-911d15fe12a9 // indirect
	github.com/klauspost/compress v1.10.3
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/libp2p/go-libp2p v0.6.0
	github.com/libp2p/go-libp2p-autonat-svc v0.1.0
	github.com/libp2p/go-libp2p-core v0.5.0
	github.com/libp2p/go-libp2p-discovery v0.2.0
	github.com/libp2p/go-libp2p-kad-dht v0.5.1
	github.com/libp2p/go-reuseport v0.0.1
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/miekg/dns v1.1.16 // indirect
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multiaddr-net v0.1.3
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/polydawn/refmt v0.0.0-20190807091052-3d65705ee9f1 // indirect
	github.com/prometheus/client_golang v1.5.0
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/rogpeppe/rjson v0.0.0-20151026200957-77220b71d327
	github.com/shirou/gopsutil v2.20.3+incompatible
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/status-im/keycard-go v0.0.0-20191119114148-6dd40a46baa0 // indirect
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/iavl v0.13.3
	github.com/tendermint/tendermint v0.33.3
	github.com/tendermint/tm-db v0.5.1
	github.com/whyrusleeping/cbor-gen v0.0.0-20200223203819-95cdfde1438f // indirect
	gitlab.com/vocdoni/go-external-ip v0.0.0-20190919225616-59cf485d00da
	go.etcd.io/bbolt v1.3.4 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	go.uber.org/zap v1.14.0
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	golang.org/x/text v0.3.2
	golang.org/x/tools v0.0.0-20200312194400-c312e98713c2 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
