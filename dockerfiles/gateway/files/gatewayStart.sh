#!/bin/sh

GWARGS="\
 ${GW_CFG_PATH:+ --cfgpath ${GW_CFG_PATH}}\
 ${GW_DATA_DIR:+ --dataDir ${GW_DATA_DIR}} \
 ${GW_LOGLEVEL:+ --logLevel ${GW_LOGLEVEL}}\
 ${GW_SIGNING_KEY:+ --signingKey ${GW_SIGNING_KEY}}\
 ${GW_SSL_DOMAIN:+ --sslDomain ${GW_SSL_DOMAIN}}\
 ${GW_ALLOW_PRIVATE:+ --allowPrivate}\
 ${GW_ALLOWED_ADDRS:+ --allowedAddrs ${GW_ALLOWED_ADDRS}}\
 ${GW_LISTEN_HOST:+ --listenHost ${GW_LISTEN_HOST}}\
 ${GW_LISTEN_PORT:+ --listenPort ${GW_LISTEN_PORT}}\
 ${GW_FILE_ENABLED:+ --fileApi=${GW_FILE_ENABLED}}\
 ${GW_CENSUS_ENABLED:+ --censusApi=${GW_CENSUS_ENABLED}}\
 ${GW_VOTE_ENABLED:+ --voteApi=${GW_VOTE_ENABLED}}\
 ${GW_API_ROUTE:+ --apiRoute=${GW_API_ROUTE}}\
 ${GW_W3_ENABLED:+ --web3Api}\
 ${GW_W3_EXTERNAL:+ --w3external ${GW_W3_EXTERNAL}}\
 ${GW_W3_ROUTE:+ --w3route ${GW_W3_ROUTE}}\
 ${GW_W3node_PORT:+ --w3nodePort ${GW_W3node_PORT}}\
 ${GW_W3_CHAIN:+ --chain ${GW_W3_CHAIN}}\
 ${GW_IPFS_NOINIT:+ --ipfsNoInit=${GW_IPFS_NOINIT}}\
 ${GW_IPFS_CLUSTER_KEY:+ --ipfsClusterKey ${GW_IPFS_CLUSTER_KEY}}\
 ${GW_IPFS_CLUSTER_PEERS:+ --ipfsClusterPeers ${GW_IPFS_CLUSTER_PEERS}}\
 ${GW_VOCHAINADDRESS:+ --vochainAddress ${GW_VOCHAINADDRESS}}\
 ${GW_VOCHAINGENESIS:+ --vochainGenesis ${GW_VOCHAINGENESIS}}\
 ${GW_VOCHAINCONTRACT:+ --vochainContract ${GW_VOCHAINCONTRACT}}\
 ${GW_VOCHAINLISTEN:+ --vochainListen ${GW_VOCHAINLISTEN}}\
 ${GW_VOCHAINLOGLEVEL:+ --vochainLogLevel ${GW_VOCHAINLOGLEVEL}}\
 ${GW_VOCHAINPEERS:+ --vochainPeers ${GW_VOCHAINPEERS}}\
 ${GW_VOCHAINRPCLISTEN:+ --vochainRPClisten ${GW_VOCHAINRPCLISTEN}}\
 ${GW_VOCHAINSEEDS:+ --vochainSeeds ${GW_VOCHAINSEEDS}}\
 ${GW_IPFSSYNCKEY:+ --ipfsSyncKey ${GW_IPFSSYNCKEY}}\
 ${GW_IPFSSYNCPEERS:+ --ipfsSyncPeers ${GW_IPFSSYNCPEERS}}\
 "

CMD="/app/gateway $GWARGS $@"
echo "Executing $CMD"
$CMD
