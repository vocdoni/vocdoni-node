#!/bin/sh

GWARGS="\
 ${gw_allowPrivate:+ --allowPrivate}\
 ${gw_allowedAddrs:+ --allowedAddrs ${gw_allowedAddrs}}\
 ${gw_apiRoute:+ --apiRoute ${gw_apiRoute}}\
 ${gw_censusApi:+ --censusApi}\
 ${gw_cfgpath:+ --cfgpath ${gw_cfgpath}}\
 ${gw_chain:+ --chain ${gw_chain}}\
 ${gw_chainLightMode:+ --chainLightMode ${gw_chainLightMode}}\
 ${gw_dataDir:+ --dataDir ${gw_dataDir}}\
 ${gw_fileApi:+ --fileApi}\
 ${gw_ipfsNoInit:+ --ipfsNoInit}\
 ${gw_ipfsSyncKey:+ --ipfsSyncKey ${gw_ipfsSyncKey}}\
 ${gw_ipfsSyncPeers:+ --ipfsSyncPeers ${gw_ipfsSyncPeers}}\
 ${gw_listenHost:+ --listenHost ${gw_listenHost}}\
 ${gw_listenPort:+ --listenPort ${gw_listenPort}}\
 ${gw_logLevel:+ --logLevel ${gw_logLevel}}\
 ${gw_logOutput:+ --logOutput ${gw_logOutput}}\
 ${gw_signingKey:+ --signingKey ${gw_signingKey}}\
 ${gw_sslDomain:+ --sslDomain ${gw_sslDomain}}\
 ${gw_vochainAddress:+ --vochainAddress ${gw_vochainAddress}}\
 ${gw_vochainContract:+ --vochainContract ${gw_vochainContract}}\
 ${gw_vochainCreateGenesis:+ --vochainCreateGenesis ${gw_vochainCreateGenesis}}\
 ${gw_vochainGenesis:+ --vochainGenesis ${gw_vochainGenesis}}\
 ${gw_vochainListen:+ --vochainListen ${gw_vochainListen}}\
 ${gw_vochainLogLevel:+ --vochainLogLevel ${gw_vochainLogLevel}}\
 ${gw_vochainPeers:+ --vochainPeers ${gw_vochainPeers}}\
 ${gw_vochainRPClisten:+ --vochainRPClisten ${gw_vochainRPClisten}}\
 ${gw_vochainSeeds:+ --vochainSeeds ${gw_vochainSeeds}}\
 ${gw_voteApi:+ --voteApi}\
 ${gw_w3WSHost:+ --w3WSHost ${gw_w3WSHost}}\
 ${gw_w3WSPort:+ --w3WSPort ${gw_w3WSPort}}\
 ${gw_w3external:+ --w3external ${gw_w3external}}\
 ${gw_w3nodePort:+ --w3nodePort ${gw_w3nodePort}}\
 ${gw_w3route:+ --w3route ${gw_w3route}}\
 ${gw_web3Api:+ --web3Api}\
"

CMD="/app/gateway $GWARGS $@"
echo "Executing $CMD"
$CMD
