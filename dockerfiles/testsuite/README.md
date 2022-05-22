# Test Suite

To run the tests:

```
docker-compose build
docker-compose up -d
go run ../../cmd/vochaintest/vochaintest.go --oracleKey $(. env.oracle0key; echo $DVOTE_ETHCONFIG_SIGNINGKEY) --electionSize=1000
```

there's also a bash script
```
./start-test.sh
```

## Default testnet components

the testnet is composed of:

 * one [seed node](https://docs.tendermint.com/master/nodes/#seed-nodes)
 * seven [miners](https://docs.vocdoni.io/architecture/services/vochain.html#miner) (aka [validator nodes](https://docs.tendermint.com/master/nodes/#validators) in tendermint jargon)
 * one [oracle](https://docs.vocdoni.io/architecture/components.html#oracle)
 * one [gateway](https://docs.vocdoni.io/architecture/components.html#gateway)

the `genesis.json` file lists the public keys of all the miners, since vochain is a Proof-of-Authority.
it also specifies which nodes are trusted oracles.

the seed node will serve to bootstrap the network: it'll just wait for incoming connections from other nodes, and provide them a list of peers which they can connect to.
the miners will first connect to the seed node, get the list of peers, and connect to each other. when there are at least 4 miners online, they can reach consensus and start producing blocks.

when the network is up and running, the tool `vochaintest` is used to simulate a voting process, interacting with the gateway node. To create the voting process (something only the oracles are entitled to do), `vochaintest` needs to know the private key of the oracle (passed in `--oracleKey`), in order to sign the transaction.

## Developing
### Connecting to docker-compose network to run vochaintest locally
Gateway is exposed on `localhost:9090`.
```
$ cd dockerfiles/testsuite && docker-compose up
vochaintest --oracleKey=... --treasurerKey=... --gwHost=http://localhost:9090/dvote --operation=tokentransactions
```
The oracle and treasurer keys are in the `env.oracle0key` and `env.treasurerkey` files.


### Adding integration tests
When adding a new integration test to `start_test.sh`, please name the container after your test. Example:
```
merkle_vote_plaintext() {
	merkle_vote poll-vote
}
...
merkle_vote() {
	$COMPOSE_CMD_RUN --name ${FUNCNAME[0]}-$1 test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
    ...
}
```
`${FUNCNAME[0]}` is the name of the current bash function, and `$1` is the argument passed to it. The container's name will be `merkle_vote-poll-vote`.

### Debugging failures
When tests fail, the logs is too polluted with output from miners to be useful. By passing `CLEAN=0` as an envvar, the docker containers are not deleted after the script finishes and you can inspect their logs with `docker logs <container name>`

```
CLEAN=0 ./start_test.sh
docker ps -a
docker logs <container name>
```

## Generate custom testnet

if you want to generate a custom-sized testnet (with X miners, Y gateways, Z oracles, and so on), check the `ansible` directory:
```sh
cd ansible
cat README.md
ansible-playbook generate_testnet.yml
../start-test.sh
```

## Troubleshooting

if you run this interactively in a headless environment (remote server), you might face the following error:

```
failed to solve with frontend dockerfile.v0: failed to solve with frontend gateway.v0: rpc error: code = Unknown desc = error getting credentials - err: exit status 1, out: `Cannot autolaunch D-Bus without X11 $DISPLAY`
```
this means `docker login` is not finding `pass` command (i.e. it's not installed in your server) and falling back to launching a d-bus server, which fails.
you can simply install `pass`, or an even more simple workaround is to make a dummy symlink
```
# ln -s /bin/true /usr/local/bin/pass
```
since `docker login` just checks that `pass` is available, but doesn't actually need it for login, all of the repos accesed are public.
