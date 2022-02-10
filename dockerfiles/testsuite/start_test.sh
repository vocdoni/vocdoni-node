#!/bin/bash
set -x
# bash start_test.sh <0|1|2|3|4>
#  0: run all tests <default>
#  1: run poll vote test
#  2: run encrypted vote test
#  3: run anonymous vote test
#  4: run csp vote test
#  5: run token transactions test (end-user voting is not included)

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1
ORACLE_KEY=${TESTSUITE_ORACLE_KEY:-6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223}
TREASURER_KEY=${TESTSUITE_TREASURER_KEY:-6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223}
ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-300}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
TEST=${1:-0}
CLEAN=${CLEAN:-1}
LOGLEVEL=${LOGLEVEL:-info}
test() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE --gwHost http://gateway0:9090/dvote --logLevel=$LOGLEVEL --operation=vtest --electionType=$1 --withWeight=2
	echo $? >$2
}

test_anon() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE_ANON --gwHost http://gateway0:9090/dvote --logLevel=$LOGLEVEL --operation=anonvoting
	echo $? >$1
}

test_csp() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE --gwHost http://gateway0:9090/dvote --logLevel=$LOGLEVEL --operation=cspvoting --electionType=$1
	echo $? >$1
}

test_token_transactions() {
	docker-compose run test timeout 300 ./vochaintest --treasurerKey=$TREASURER_KEY --gwHost http://gateway0:9090/dvote --logLevel=$LOGLEVEL --operation=tokentransactions
	echo $? >$1
}

echo "### Starting test suite ###"
docker-compose build
docker-compose up -d

echo "### Waiting for test suite to be ready ###"
for i in {1..20}; do
	docker-compose run test curl -s --fail http://gateway0:9090/dvote \
		-X POST \
		-d '{"id": "req00'$RANDOM'", "request": {"method": "genProof", "timestamp":'$(date +%s)'}}' 2>/dev/null && break || sleep 2
done

. env.oracle0key
ORACLE_KEY="$DVOTE_ETHCONFIG_SIGNINGKEY"
if [ -n "$TESTSUITE_ORACLE_KEY" ] ; then
	ORACLE_KEY="$TESTSUITE_ORACLE_KEY"
fi

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
	test test_anon ${testid}3 &
} || echo 0 >${testid}3

[ $TEST -eq 4 -o $TEST -eq 0 ] && {
	echo "### Running test 4 ###"
	test test_csp ${testid}4 &
} || echo 0 >${testid}4

[ $TEST -eq 5 -o $TEST -eq 0 ] && {
	echo "### Running test 5 ###"
	test test_token_transactions ${testid}5 &
} || echo 0 >${testid}5

echo "### Waiting for tests to finish ###"
wait

[ "$(cat ${testid}1)" == "0" -a "$(cat ${testid}2)" == "0" -a "$(cat ${testid}3)" == "0" \
  -a "$(cat ${testid}4)" == "0" -a "$(cat ${testid}5)" == "0" ] && {
	echo "Vochain test finished correctly!"
	RET=0
} || {
	echo "Vochain test failed!"
	RET=1
	echo "### Post run logs ###"
	docker-compose logs --tail 1000
}

[ $CLEAN -eq 1 ] && {
	echo "### Cleaning environment ###"
	docker-compose down -v --remove-orphans
}

rm -f ${testid}1 ${testid}2 ${testid}3 ${testid}4 ${testid}5
exit $RET
