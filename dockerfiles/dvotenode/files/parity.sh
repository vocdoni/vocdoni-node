#!/bin/sh

# If no IPC configured as web3External, do nothing
[ "$DVOTE_W3CONFIG_W3EXTERNAL" != "/app/eth/jsonrpc.ipc" ] && {
	# trick for avoid restart loop on docker-compose
	[ "$1" == "always" ] && while true; do sleep 5; done
	exit 0
}

CHAIN=${DVOTE_ETHCONFIG_CHAINTYPE:-sokol}
[ "$CHAIN" == "mainnet" ] && CHAIN="ethereum"

/bin/parity --chain $CHAIN --base-path /app/eth --port=37671 \
	--ipc-apis=all --jsonrpc-port=9080 --jsonrpc-interface=0.0.0.0 --jsonrpc-apis=safe \
	--jsonrpc-cors=all --ws-port=9081 --ws-interface=0.0.0.0 --ws-origins=* --no-ancient-blocks
