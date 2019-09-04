module gitlab.com/vocdoni/go-dvote

go 1.12

require (
	github.com/apilayer/freegeoip v3.5.0+incompatible // indirect
	github.com/cespare/cp v1.1.1 // indirect
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/decred/dcrd/dcrec/secp256k1 v1.0.2
	github.com/docker/docker v1.13.1 // indirect
	github.com/ethereum/go-ethereum v1.9.3
	github.com/ethersphere/swarm v0.4.3
	github.com/fjl/memsize v0.0.0-20190710130421-bcb5799ab5e5 // indirect
	github.com/gobwas/httphead v0.0.0-20180130184737-2c6c146eadee // indirect
	github.com/gobwas/pool v0.2.0 // indirect
	github.com/gobwas/ws v1.0.2
	github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6 // indirect
	github.com/gorilla/websocket v1.4.1
	github.com/graph-gophers/graphql-go v0.0.0-20190902214650-641ae197eec7 // indirect
	github.com/hashicorp/go-uuid v1.0.1 // indirect
	github.com/howeyc/fsnotify v0.9.0 // indirect
	github.com/hsanjuan/go-libp2p-http v0.0.2 // indirect
	github.com/iden3/go-iden3-core v0.0.7-0.20190904124812-741e2fdbb8f0
	github.com/influxdata/influxdb v1.7.7 // indirect
	github.com/ipfs/dir-index-html v1.0.3 // indirect
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.0
	github.com/ipfs/go-fs-lock v0.0.1
	github.com/ipfs/go-ipfs v0.4.22-0.20190903225735-3c04d7423817
	github.com/ipfs/go-ipfs-addr v0.0.1 // indirect
	github.com/ipfs/go-ipfs-api v0.0.1
	github.com/ipfs/go-ipfs-config v0.0.11
	github.com/ipfs/go-ipfs-files v0.0.3
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/interface-go-ipfs-core v0.2.2
	github.com/ipfs/ipfs-cluster v0.11.0-rc8.0.20190829153225-ef8483e30e42
	github.com/libp2p/go-conn-security v0.0.1 // indirect
	github.com/libp2p/go-libp2p-core v0.2.2
	github.com/libp2p/go-libp2p-host v0.1.0
	github.com/libp2p/go-libp2p-interface-connmgr v0.0.5 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.2.0
	github.com/libp2p/go-libp2p-net v0.0.2 // indirect
	github.com/libp2p/go-libp2p-peer v0.2.0
	github.com/libp2p/go-libp2p-pubsub v0.1.1
	github.com/libp2p/go-libp2p-transport v0.0.5 // indirect
	github.com/libp2p/go-stream-muxer v0.1.0 // indirect
	github.com/libp2p/go-testutil v0.1.0 // indirect
	github.com/mailru/easyjson v0.0.0-20190221075403-6243d8e04c3f // indirect
	github.com/marcusolsson/tui-go v0.4.0
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/oschwald/maxminddb-golang v1.4.0 // indirect
	github.com/peterh/liner v1.1.0 // indirect
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/robertkrimen/otto v0.0.0-20180617131154-15f95af6e78d // indirect
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/uber-go/atomic v1.4.0 // indirect
	github.com/uber/jaeger-client-go v2.17.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.0.0+incompatible // indirect
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	go.opencensus.io v0.22.0
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190820162420-60c769a6c586
	golang.org/x/sys v0.0.0-20190826190057-c7b8b68b1456
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20190709231704-1e4459ed25ff // indirect
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/urfave/cli.v1 v1.0.0-00010101000000-000000000000 // indirect
)

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0

replace gopkg.in/urfave/cli.v1 => github.com/urfave/cli v1.21.0

replace github.com/go-critic/go-critic v0.0.0-20181204210945-1df300866540 => github.com/go-critic/go-critic v0.3.4

replace github.com/golangci/go-tools v0.0.0-20180109140146-af6baa5dc196 => github.com/golangci/go-tools v0.0.0-20190318060251-af6baa5dc196

replace github.com/golangci/gofmt v0.0.0-20181105071733-0b8337e80d98 => github.com/golangci/gofmt v0.0.0-20181222123516-0b8337e80d98

replace github.com/golangci/gosec v0.0.0-20180901114220-66fb7fc33547 => github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547

replace github.com/golangci/ineffassign v0.0.0-20180808204949-42439a7714cc => github.com/golangci/ineffassign v0.0.0-20190609212857-42439a7714cc

replace github.com/golangci/lint-1 v0.0.0-20180610141402-ee948d087217 => github.com/golangci/lint-1 v0.0.0-20190420132249-ee948d087217

replace mvdan.cc/unparam v0.0.0-20190124213536-fbb59629db34 => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34

replace github.com/golangci/errcheck v0.0.0-20181003203344-ef45e06d44b6 => github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6
