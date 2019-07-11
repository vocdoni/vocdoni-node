#!/bin/sh

GWARGS="\
 ${GW_CFG_PATH:+ --cfgpath ${GW_CFG_PATH}}\
 ${GW_DATA_DIR:+ --dataDir ${GW_DATA_DIR}} \
 ${GW_LOGLEVEL:+ --loglevel ${GW_LOGLEVEL}}\
 ${GW_SIGNING_KEY:+ --signingKey ${GW_SIGNING_KEY}}\
 ${GW_SSL_DOMAIN:+ --sslDomain ${GW_SSL_DOMAIN}}\
 ${GW_ALLOW_PRIVATE:+ --allowPrivate ${GW_ALLOW_PRIVATE}}\
 ${GW_ALLOWED_ADDRS:+ --allowedAddrs ${GW_ALLOWED_ADDRS}}\
 ${GW_DVOTE_ENABLED:+ --fileApi ${GW_DVOTE_ENABLED}}\
 ${GW_DVOTE_HOST:+ --dvoteHost ${GW_DVOTE_HOST}}\
 ${GW_DVOTE_PORT:+ --dvotePort ${GW_DVOTE_PORT}}\
 ${GW_DVOTE_PATH:+ --dvoteRoute ${GW_DVOTE_PATH}}\
 ${GW_W3_ENABLED:+ --web3Api ${GW_W3_ENABLED}}\
 ${GW_W3_ROUTE:+ --w3Route ${GW_W3_ROUTE}}\
 ${GW_W3node_PORT:+ --w3nodePort ${GW_W3node_PORT}}\
 ${GW_W3_WS_PORT:+ --w3wsPort ${GW_W3_WS_PORT}}\
 ${GW_W3_WS_HOST:+ --w3wsHost ${GW_W3_WS_HOST}}\
 ${GW_W3_HTTP_PORT:+ --w3httpPort ${GW_W3_HTTP_PORT}}\
 ${GW_W3_HTTP_HOST:+ --w3httpHost ${GW_W3_HTTP_HOST}}\
 ${GW_W3_CHAIN:+ --chain ${GW_W3_CHAIN}}\
 ${GW_IPFS_DAEMON_PATH:+ --ipfsDaemon ${GW_IPFS_DAEMON_PATH}}\
 ${GW_IPFS_NO_INIT:+ --ipfsNoInit ${GW_IPFS_NO_INIT}}\
 "

ipfs init 1>/dev/null 2>&1
CMD="/app/gateway $GWARGS $@"
echo "Executing $CMD"
$CMD
