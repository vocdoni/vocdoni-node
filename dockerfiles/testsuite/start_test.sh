#!/bin/bash
# bash start_test.sh <0|1|2>
#  0: run all tests <default>
#  1: run poll vote test
#  2: run encrypted vote test

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1
ORACLE_KEY=${TESTSUITE_ORACLE_KEY:-6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223}
ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-300}
TEST=${1:-0}
CLEAN=${CLEAN:-1}

test() {
	docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE --gwHost ws://gateway:9090/dvote --logLevel=INFO --electionType=$1
	echo $? >$2
}

echo "### Starting test suite ###"
docker-compose build
docker-compose up -d

sleep 5
echo "### Waiting for test suite to be ready ###"
for i in {1..5}; do
	docker-compose run test curl --fail http://gateway:9090/dvote \
		-X POST \
		-d '{"id": "req00'$RANDOM'", "request": {"method": "getInfo", "timestamp":'$(date +%s)'}}' && break || sleep 5
done

testid="/tmp/.vochaintest$RANDOM"

[ $TEST -eq 1 -o $TEST -eq 0 ] && {
	echo "### Running test 1 ###"
	test poll-vote ${testid}1 &
} || echo 0 >${testid}1

[ $TEST -eq 2 -o $TEST -eq 0 ] && {
	echo "### Running test 2 ###"
	test encrypted-poll ${testid}2 &
} || echo 0 >${testid}2

echo "### Waiting for tests ###"
wait

#echo "### Post run logs ###"
#docker-compose logs --tail 300

[ $CLEAN -eq 1 ] && {
	echo "### Cleaning environment ###"
	docker-compose down -v --remove-orphans
}

[ "$(cat ${testid}1)" == "0" -a "$(cat ${testid}2)" == "0" ] && {
	echo "Vochain test finished correctly!"
	RET=0
} || {
	echo "Vochain test failed!"
	RET=1
}
rm -f ${testid}1 ${testid}2
exit $RET
