#!/bin/sh

GWARGS="\
 ${gw_apiAllowPrivate:+ --apiAllowPrivate}\
 ${gw_apiAllowedAddrs:+ --apiAllowedAddrs=${gw_apiAllowedAddrs}}\
 ${gw_apiRoute:+ --apiRoute=${gw_apiRoute}}\
 ${gw_censusApi:+ --censusApi}\
 ${gw_censusSync:+ --censusSync=${gw_censusSync}}\
 ${gw_contract:+ --contract=${gw_contract}}\
 ${gw_ethChain:+ --ethChain=${gw_ethChain}}\
 ${gw_ethChainLightMode:+ --ethChainLightMode=${gw_ethChainLightMode}}\
 ${gw_ethBootNodes:+ --ethBootNodes=${gw_ethBootNodes}}\
 ${gw_dataDir:+ --dataDir=${gw_dataDir}}\
 ${gw_fileApi:+ --fileApi}\
 ${gw_ipfsNoInit:+ --ipfsNoInit}\
 ${gw_ipfsSyncKey:+ --ipfsSyncKey=${gw_ipfsSyncKey}}\
 ${gw_ipfsSyncPeers:+ --ipfsSyncPeers=${gw_ipfsSyncPeers}}\
 ${gw_listenHost:+ --listenHost=${gw_listenHost}}\
 ${gw_listenPort:+ --listenPort=${gw_listenPort}}\
 ${gw_logLevel:+ --logLevel=${gw_logLevel}}\
 ${gw_logOutput:+ --logOutput=${gw_logOutput}}\
 ${gw_ethSigningKey:+ --ethSigningKey=${gw_ethSigningKey}}\
 ${gw_sslDomain:+ --sslDomain=${gw_sslDomain}}\
 ${gw_sslDirCert:+ --sslDirCert=${gw_sslDirCert}}\
 ${gw_vochainPublicAddr:+ --vochainPublicAddr=${gw_vochainPublicAddr}}\
 ${gw_vochainCreateGenesis:+ --vochainCreateGenesis=${gw_vochainCreateGenesis}}\
 ${gw_vochainGenesis:+ --vochainGenesis=${gw_vochainGenesis}}\
 ${gw_vochainP2PListen:+ --vochainP2PListen=${gw_vochainP2PListen}}\
 ${gw_vochainLogLevel:+ --vochainLogLevel=${gw_vochainLogLevel}}\
 ${gw_vochainPeers:+ --vochainPeers=${gw_vochainPeers}}\
 ${gw_vochainRPCListen:+ --vochainRPCListen=${gw_vochainRPCListen}}\
 ${gw_vochainSeeds:+ --vochainSeeds=${gw_vochainSeeds}}\
 ${gw_voteApi:+ --voteApi}\
 ${gw_w3WsHost:+ --w3WsHost=${gw_w3WsHost}}\
 ${gw_w3WsPort:+ --w3WsPort=${gw_w3WsPort}}\
 ${gw_w3HTTPHost:+ --w3HTTPHost=${gw_w3HTTPHost}}\
 ${gw_w3HTTPPort:+ --w3HTTPPort=${gw_w3HTTPPort}}\
 ${gw_ethNodePort:+ --ethNodePort=${gw_ethNodePort}}\
 ${gw_w3route:+ --w3route=${gw_w3route}}\
 ${gw_w3Enabled:+ --w3Enabled}\
"

CMD="/app/gateway $GWARGS $@"
echo "Executing $CMD"
$CMD
