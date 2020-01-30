#!/bin/sh

MINERARGS="\
 ${miner_dataDir:+ --dataDir=${miner_dataDir}}\
 ${miner_logLevel:+ --logLevel=${miner_logLevel}}\
 ${miner_logOutput:+ --logOutput=${miner_logOutput}}\
 ${miner_publicAddr:+ --publicAddr=${miner_publicAddr}}\
 ${miner_minerKeyFile:+ --minerKeyFile=${miner_minerKeyFile}}\
 ${miner_keyFile:+ --keyFile=${miner_keyFile}}\
 ${miner_genesis:+ --genesis=${miner_genesis}}\
 ${miner_p2pListen:+ --p2pListen=${miner_p2pListen}}\
 ${miner_peers:+ --peers=${miner_peers}}\
 ${miner_rpcListen:+ --rpcListen=${miner_rpcListen}}\
 ${miner_seeds:+ --seeds=${miner_seeds}}\
"

CMD="/app/miner $MINERARGS $@"
echo "Executing $CMD"
$CMD
