#!/bin/sh

ORACLEARGS="\
 ${oracle_contract:+ --contract=${oracle_contract}}\
 ${oracle_ethChain:+ --ethChain=${oracle_ethChain}}\
 ${oracle_ethChainLightMode:+ --ethChainLightMode=${oracle_ethChainLightMode}}\
 ${oracle_dataDir:+ --dataDir=${oracle_dataDir}}\
 ${oracle_ethSigningKey:+ --ethSigningKey=${oracle_ethSigningKey}}\
 ${oracle_ethNodePort:+ --ethNodePort=${oracle_ethNodePort}}\
 ${oracle_ethBootNodes:+ --ethBootNodes=${oracle_ethBootNodes}}\
 ${oracle_ethTrustedPeers:+ --ethTrustedPeers=${oracle_ethTrustedPeers}}\
 ${oracle_logLevel:+ --logLevel=${oracle_logLevel}}\
 ${oracle_logOutput:+ --logOutput=${oracle_logOutput}}\
 ${oracle_subscribeOnly:+ --subscribeOnly=${oracle_subscribeOnly}}\
 ${oracle_vochainPublicAddr:+ --vochainPublicAddr=${oracle_vochainPublicAddr}}\
 ${oracle_vochainKeyFile:+ --vochainKeyFile=${oracle_vochainKeyFile}}\
 ${oracle_vochainGenesis:+ --vochainGenesis=${oracle_vochainGenesis}}\
 ${oracle_vochainP2PListen:+ --vochainP2PListen=${oracle_vochainP2PListen}}\
 ${oracle_vochainLogLevel:+ --vochainLogLevel=${oracle_vochainLogLevel}}\
 ${oracle_vochainPeers:+ --vochainPeers=${oracle_vochainPeers}}\
 ${oracle_vochainRPCListen:+ --vochainRPCListen=${oracle_vochainRPCListen}}\
 ${oracle_vochainSeeds:+ --vochainSeeds=${oracle_vochainSeeds}}\
 ${oracle_w3WsHost:+ --w3WsHost=${oracle_w3WsHost}}\
 ${oracle_w3WsPort:+ --w3WsPort=${oracle_w3WsPort}}\
 ${oracle_w3HTTPHost:+ --w3HTTPHost=${oracle_w3HTTPHost}}\
 ${oracle_w3HTTPPort:+ --w3HTTPPort=${oracle_w3HTTPPort}}\
"

CMD="/app/oracle $ORACLEARGS $@"
echo "Executing $CMD"
$CMD
