#!/bin/sh

ARGS="\
 ${CS_CFG_PATH:+ --cfgpath ${CS_CFG_PATH}}\
 ${CS_DATA_DIR:+ --dataDir ${CS_DATA_DIR}} \
 ${CS_LOGLEVEL:+ --logLevel ${CS_LOGLEVEL}}\
 ${CS_SIGNING_KEY:+ --signKey ${CS_SIGNING_KEY}}\
 ${CS_ROOT_KEY:+ --rootKey ${CS_ROOT_KEY}}\
 ${CS_SSL_DOMAIN:+ --sslDomain ${CS_SSL_DOMAIN}}\
 ${CS_LISTEN_PORT:+ --port ${CS_LISTEN_PORT}}\
 "

CMD="/app/census $ARGS $@"
echo "Executing $CMD"
$CMD
