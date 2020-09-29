#!/bin/sh

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1
ORACLE_KEY=${TESTSUITE_ORACLE_KEY:-6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223}
ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-1000}

echo "### Starting test suite ###"
docker-compose up -d

echo "### Waiting for test suite to be ready ###"
for i in {1..5}; do docker-compose run test curl --fail http://gateway:9090/ping && break || sleep 5; done

echo "### Running test ###"
docker-compose run test timeout 300 ./vochaintest --oracleKey=$ORACLE_KEY --electionSize=$ELECTION_SIZE --gwHost ws://gateway:9090/dvote --logLevel=INFO
RETVAL=$?

echo "### Post run logs ###"
docker-compose logs --tail 300

echo "### Cleaning environment ###"
docker-compose down -v --remove-orphans

exit $RETVAL
