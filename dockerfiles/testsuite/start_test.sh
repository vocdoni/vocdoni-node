#!/bin/sh

echo "### Starting test suite ###"
docker-compose up -d

echo "### Pre run logs ###"
docker-compose logs

echo "### Waiting for test suite to be ready ###"
for i in {1..5}; do docker-compose run test curl --fail http://gateway:9090/ping && break || sleep 5; done

echo "### Running test ###"
docker-compose run test timeout 300 ./vochaintest --oracleKey=$TESTSUITE_ORACLE_KEY --electionSize=$TESTSUITE_ELECTION_SIZE --gwHost ws://gateway:9090/dvote

echo "### Post run logs ###"
docker-compose logs --tail 50

echo "### Cleaning environment ###"
docker-compose down -v --remove-orphans
