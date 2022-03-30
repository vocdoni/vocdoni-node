#!/bin/bash
# bash start_test.sh [testname] [testname] [...]
#  (if no argument is passed, run all tests)
#  merkle_vote_plaintext: run poll vote test
#  merkle_vote_encrypted: run encrypted vote test
#  anonvoting: run anonymous vote test
#  cspvoting: run csp vote test
#  tokentransactions: run token transactions test (end-user voting is not included)

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1
ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-300}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
CLEAN=${CLEAN:-1}
LOGLEVEL=${LOGLEVEL:-info}
GWHOST="http://gateway0:9090/dvote"
. env.oracle0key # contains var DVOTE_ETHCONFIG_ORACLEKEY, import into current env
ORACLE_KEY="$DVOTE_ETHCONFIG_SIGNINGKEY"
[ -n "$TESTSUITE_ORACLE_KEY" ] && ORACLE_KEY="$TESTSUITE_ORACLE_KEY"
. env.treasurerkey # contains var DVOTE_ETHCONFIG_SIGNINGKEY, import into current env
TREASURER_KEY="$DVOTE_ETHCONFIG_SIGNINGKEY"
[ -n "$TESTSUITE_TREASURER_KEY" ] && TREASURER_KEY="$TESTSUITE_TREASURER_KEY"
. env.entity0key # contains var DVOTE_ETHCONFIG_SIGNINGKEY, import into current env
ENTITY_KEY="$DVOTE_ETHCONFIG_SIGNINGKEY"
[ -n "$TESTSUITE_ENTITY_KEY" ] && ENTITY_KEY="$TESTSUITE_ENTITY_KEY"

### if you want to add a new test:
### add "newtest" to tests_to_run array (as well as a comment at the head of the file)
### and a new function with exactly that name:
### newtest() { whatever ; }

tests_to_run=(
	"merkle_vote_plaintext"
	"merkle_vote_encrypted"
	"anonvoting"
	"cspvoting"
	"tokentransactions"
)

# if any arg is passed, treat them as the tests to run, overriding the default list
[ $# != 0 ] && tests_to_run=($@)

merkle_vote() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=vtest \
		  --oracleKey=$ORACLE_KEY \
		  --electionSize=$ELECTION_SIZE \
		  --electionType=$1 \
		  --withWeight=2
}

merkle_vote_plaintext() {
	merkle_vote poll-vote
}

merkle_vote_encrypted() {
	merkle_vote encrypted-poll
}

anonvoting() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=anonvoting \
		  --oracleKey=$ORACLE_KEY \
		  --electionSize=$ELECTION_SIZE_ANON
}

cspvoting() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=cspvoting \
		  --oracleKey=$ORACLE_KEY \
		  --electionSize=$ELECTION_SIZE
}

tokentransactions() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=tokentransactions \
		  --oracleKey=$ORACLE_KEY \
		  --treasurerKey=$TREASURER_KEY \
		  --entityKey=$ENTITY_KEY
}

### end tests definition

# useful for debugging bash flow
# docker-compose() { echo "# would do: docker-compose $@" ; sleep 0.2 ;}

echo "### Starting test suite ###"
docker-compose build
docker-compose up -d

check_gw_is_up() {
	docker-compose run test \
		curl -s --fail $GWHOST \
		  -X POST \
		  -d '{"id": "req00'$RANDOM'", "request": {"method": "genProof", "timestamp":'$(date +%s)'}}' 2>/dev/null
}

echo "### Waiting for test suite to be ready ###"
for i in {1..20}; do
	if check_gw_is_up ; then break ; else sleep 2 ; fi
done

# create temp dir
results="/tmp/.vochaintest$RANDOM"
mkdir -p $results

for test in ${tests_to_run[@]}; do
	echo "### Running test $test ###"
	( $test ; echo $? > $results/$test )
done

echo "### Waiting for tests to finish ###"
wait

for test in ${tests_to_run[@]} ; do
	RET=$(cat $results/$test)
	if [ "$RET" == "0" ] ; then
		echo "Vochain test $test finished correctly!"
	else
		echo "Vochain test $test failed!"
		echo "### Post run logs ###"
		docker-compose logs --tail 1000
		break
	fi
done

# remove temp dir
rm -rf $results

[ $CLEAN -eq 1 ] && {
	echo "### Cleaning docker environment ###"
	docker-compose down -v --remove-orphans
}

exit $RET
