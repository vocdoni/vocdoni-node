# Test Suite

To run the tests:

```
docker compose build
docker compose up -d
go run ../../cmd/end2endtest/
```

there's also a bash script, which prefers to be run with `NOTTY=1`
```
NOTTY=1 ./start_test.sh
```

## Default testnet components

the testnet is composed of:

 * one [seed node](https://docs.tendermint.com/master/nodes/#seed-nodes)
 * four [miners](https://docs.vocdoni.io/architecture/services/vochain.html#miner) (aka [validator nodes](https://docs.tendermint.com/master/nodes/#validators) in tendermint jargon)
 * one [gateway](https://docs.vocdoni.io/architecture/components.html#gateway)

the seed node will serve to bootstrap the network: it'll just wait for incoming connections from other nodes, and provide them a list of peers which they can connect to.
the miners will first connect to the seed node, get the list of peers, and connect to each other. when there are at least 3 miners online, they can reach consensus and start producing blocks.

when the network is up and running, the tool `end2endtest` is used to simulate a voting process, interacting with the gateway node. To create the voting process (a transaction that costs a certain amount of tokens), `end2endtest` uses a faucet offered by the testsuite (passed in `--faucet`) to fund the accounts.

## Developing
### Adding integration tests
When adding a new integration test to `start_test.sh`, please name the container after your test, including the RANDOMID to allow concurrent test runs in CI servers. Example:
```
e2etest_plaintextelection() {
	e2etest plaintextelection --votes=100
}
...
e2etest() {
	op=$1
	shift
	args=$@
	id="${op}_$(echo $args | md5sum | awk '{print $1}')"
	$COMPOSE_CMD_RUN --name ${TEST_PREFIX}_${FUNCNAME[0]}-${id}_${RANDOMID} test timeout 300 \
		./end2endtest --host $APIHOST --faucet=$FAUCET \
		  --logLevel=$LOGLEVEL \
		  --operation=$op \
		  --parallel=$CONCURRENT_CONNECTIONS \
 		  $args
}
```
`${FUNCNAME[0]}` is the name of the current bash function, and `$@` are the arguments passed to it. The container's name will be `plaintextelection-bb448c37657a78f302a6fd283671c83a`.

### Debugging failures
When tests fail, the logs is too polluted with output from miners to be useful. By passing `CLEAN=0` as an envvar, the docker containers are not deleted after the script finishes and you can inspect their logs with `docker logs <container name>`

```
NOTTY=1 CLEAN=0 ./start_test.sh
docker ps -a
docker logs <container name>
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

## Grafana

if you want to collect Prometheus metrics and visualize them in a local Grafana, enable `grafana` [profile](https://docs.docker.com/compose/profiles/):

```sh
export COMPOSE_PROFILES=grafana
docker compose up -d
```

or simply

```sh
docker compose --profile grafana up -d
```
