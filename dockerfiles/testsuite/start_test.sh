#!/bin/bash
set -x
# bash start_test.sh <0|1|2|3|4>
#  0: run all tests <default>
#  1: run poll vote test
#  2: run encrypted vote test
#  3: run anonymous vote test
#  4: run all transactions test (end-user voting is not included)

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1
ORACLE_KEY=${TESTSUITE_ORACLE_KEY:-6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223}
ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-300}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
TEST=${1:-0}
CLEAN=${CLEAN:-1}
LOGLEVEL=${LOGLEVEL:info}
test() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE --gwHost http://gateway:9090/dvote --logLevel=$LOGLEVEL --operation=vtest --electionType=$1 --withWeight=2
	echo $? >$2
}

test_anon() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE_ANON --gwHost http://gateway:9090/dvote --logLevel=$LOGLEVEL --operation=anonvoting
	echo $? >$1
}

test_txs() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --gwHost http://gateway:9090/dvote --logLevel=$LOGLEVEL --operation=testtxs
	echo $? >$1
}

echo "### Starting test suite ###"
docker-compose build
docker-compose up -d

echo "### Waiting for test suite to be ready ###"
for i in {1..5}; do
	docker-compose run test curl --fail http://gateway:9090/dvote \
		-X POST \
		-d '{"id": "req00'$RANDOM'", "request": {"method": "getInfo", "timestamp":'$(date +%s)'}}' && break || sleep 5
done
sleep 10 # extra wait to ensure all gateways API methods have been started

testid="/tmp/.vochaintest$RANDOM"

[ $TEST -eq 1 -o $TEST -eq 0 ] && {
	echo "### Running test 1 ###"
	test poll-vote ${testid}1 &
} || echo 0 >${testid}1

[ $TEST -eq 2 -o $TEST -eq 0 ] && {
	echo "### Running test 2 ###"
	test encrypted-poll ${testid}2 &
} || echo 0 >${testid}2

[ $TEST -eq 3 -o $TEST -eq 0 ] && {
	echo "### Running test 3 ###"
	test_anon ${testid}3 &
} || echo 0 >${testid}3


echo "### Waiting for voting process tests ###"
wait

echo "### Waiting for all state txs tests ###"

[ $TEST -eq 4 -o $TEST -eq 0 ] && {
	echo "### Running test 4 ###"
	test_txs ${testid}4 &
} || echo 0 >${testid}4

wait

[ $CLEAN -eq 1 ] && {
	echo "### Cleaning environment ###"
	docker-compose down -v --remove-orphans
}

[ "$(cat ${testid}1)" == "0" -a "$(cat ${testid}2)" == "0" -a "$(cat ${testid}3)" == "0" -a "$(cat ${testid}4)" == "0" ] && {
	echo "Vochain test finished correctly!"
	RET=0
} || {
	echo "Vochain test failed!"
	RET=1
	echo "### Post run logs ###"
	docker-compose logs --tail 1000
}
rm -f ${testid}1 ${testid}2 ${testid}3 ${testid}4
exit $RET
