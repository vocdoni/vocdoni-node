module gitlab.com/vocdoni/go-dvote

go 1.14

require (
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/aristanetworks/goarista v0.0.0-20200602234848-db8a79a18e4a // indirect
	github.com/btcsuite/btcd v0.21.0-beta // indirect
	github.com/deroproject/graviton v0.0.0-20200906044921-89e9e09f9601
	github.com/dgraph-io/badger/v2 v2.0.1-rc1.0.20200409094109-809725940698
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/ethereum/go-ethereum v1.9.20
	github.com/gballet/go-libpcsclite v0.0.0-20191108122812-4678299bea08 // indirect
	github.com/go-chi/chi v4.1.1+incompatible
	github.com/go-chi/cors v1.1.1
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/google/go-cmp v0.5.0
	github.com/google/gopacket v1.1.18 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190812055157-5d271430af9f // indirect
	github.com/iden3/go-iden3-core v0.0.8-0.20200325104031-1ed04a261b78
	github.com/iden3/go-iden3-crypto v0.0.4
	github.com/ipfs/go-filestore v1.0.0 // indirect
	github.com/ipfs/go-ipfs v0.6.0
	github.com/ipfs/go-ipfs-config v0.8.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-log/v2 v2.1.1 // indirect
	github.com/ipfs/interface-go-ipfs-core v0.3.0
	github.com/karalabe/usb v0.0.0-20191104083709-911d15fe12a9 // indirect
	github.com/klauspost/compress v1.10.6
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/libp2p/go-libp2p v0.10.0
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.8.2
	github.com/libp2p/go-libp2p-quic-transport v0.8.0 // indirect
	github.com/libp2p/go-netroute v0.1.3 // indirect
	github.com/libp2p/go-reuseport v0.0.1
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/polydawn/refmt v0.0.0-20190807091052-3d65705ee9f1 // indirect
	github.com/prometheus/client_golang v1.6.0
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/rogpeppe/rjson v0.0.0-20151026200957-77220b71d327
	github.com/shirou/gopsutil v2.20.5+incompatible
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.3
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/iavl v0.13.3
	github.com/tendermint/tendermint v0.33.5
	github.com/tendermint/tm-db v0.5.1
	github.com/whyrusleeping/cbor-gen v0.0.0-20200223203819-95cdfde1438f // indirect
	gitlab.com/vocdoni/go-external-ip v0.0.0-20190919225616-59cf485d00da
	go.etcd.io/bbolt v1.3.4 // indirect
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/sys v0.0.0-20200828194041-157a740278f4 // indirect
	golang.org/x/text v0.3.2
	google.golang.org/protobuf v1.25.0 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
	nhooyr.io/websocket v1.8.6
)

// Duktape is very slow to build, and can't be built with multiple cores since
// it includes a lot of C in a single file. Until
// https://github.com/ethereum/go-ethereum/issues/20590 is fixed, stub it out
// with a replace directive. The stub was hacked together with vim.
replace gopkg.in/olebedev/go-duktape.v3 => ./duktape-stub
