#!/bin/sh

GWARGS="\
 ${GW_CFG_PATH:+ --cfgpath ${GW_CFG_PATH}}\
 ${GW_DATA_DIR:+ --dataDir ${GW_DATA_DIR}} \
 ${GW_LOGLEVEL:+ --logLevel ${GW_LOGLEVEL}}\
 ${GW_SIGNING_KEY:+ --signingKey ${GW_SIGNING_KEY}}\
 ${GW_SSL_DOMAIN:+ --sslDomain ${GW_SSL_DOMAIN}}\
 ${GW_ALLOW_PRIVATE:+ --allowPrivate}\
 ${GW_ALLOWED_ADDRS:+ --allowedAddrs ${GW_ALLOWED_ADDRS}}\
 ${GW_DVOTE_ENABLED:+ --dvoteApi}\
 ${GW_LISTEN_HOST:+ --listenHost ${GW_LISTEN_HOST}}\
 ${GW_LISTEN_PORT:+ --listenPort ${GW_LISTEN_PORT}}\
 ${GW_DVOTE_PATH:+ --dvoteRoute ${GW_DVOTE_PATH}}\
 ${GW_W3_ENABLED:+ --web3Api}\
 ${GW_W3_EXTERNAL:+ --w3external ${GW_W3_EXTERNAL}}\
 ${GW_W3_ROUTE:+ --w3route ${GW_W3_ROUTE}}\
 ${GW_W3node_PORT:+ --w3nodePort ${GW_W3node_PORT}}\
 ${GW_W3_CHAIN:+ --chain ${GW_W3_CHAIN}}\
 ${GW_IPFS_DAEMON_PATH:+ --ipfsDaemon ${GW_IPFS_DAEMON_PATH}}\
 ${GW_IPFS_NO_INIT:+ --ipfsNoInit}\
 "

CMD="/app/gateway $GWARGS $@"
echo "Executing $CMD"
$CMD
