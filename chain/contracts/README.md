# Ethereum bindings

Solidity 6 is required, a static version can be found here
http://ppa.launchpad.net/ethereum/ethereum-static/ubuntu/pool/main/s/solc/solc_0.6.6-0ubuntu1~STATIC_amd64.deb


Then the bindings can be built as follows
```
solc6 --abi --bin ENSRegistry.sol -o build
abigen --bin=./build/ENSRegistry.bin --abi=./build/ENSRegistry.abi --pkg=contracts --out=ENSRegistry.go
```
