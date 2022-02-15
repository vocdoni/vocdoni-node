#!/bin/bash
# bash start_test.sh [testname] [testname] [...]
#  (if no argument is passed, run all tests)
#  merkle_vote_plaintext: run poll vote test
#  merkle_vote_encrypted: run encrypted vote test
#  anonvoting: run anonymous vote test
#  cspvoting: run csp vote test
#  tokentransactions: run token transactions test (end-user voting is not included)
#  vocli: test the CLI tool

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1
ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-300}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
CLEAN=${CLEAN:-1}
LOGLEVEL=${LOGLEVEL:-info}
CONCURRENT=${CONCURRENT:-1}
### must be splited by lines
DEFAULT_ACCOUNT_KEYS="73ac72a16ea84dd1f76b62663d2aa380253aec4386935e460dad55d4293a0b11
be9248891bd6c220d013afb4b002f72c8c22cbad9c02003c19729bcbd6962e52
cb595f3fa1a4790dd54c139524a1430fc500f95a02affee6a933fcb88849a48d
ab595f3fa1a4790dd54c139524a1430fc500f95a02affee6a933fcb88849a56a"
ACCOUNT_KEYS=${ACCOUNT_KEYS:-$DEFAULT_ACCOUNT_KEYS}
GWHOST="http://gateway0:9090/dvote"
. env.oracle0key # contains var DVOTE_ETHCONFIG_ORACLEKEY, import into current env
ORACLE_KEY="$DVOTE_ETHCONFIG_SIGNINGKEY"
[ -n "$TESTSUITE_ORACLE_KEY" ] && ORACLE_KEY="$TESTSUITE_ORACLE_KEY"
. env.treasurerkey # contains var DVOTE_ETHCONFIG_SIGNINGKEY, import into current env
TREASURER_KEY="$DVOTE_ETHCONFIG_SIGNINGKEY"
[ -n "$TESTSUITE_TREASURER_KEY" ] && TREASURER_KEY="$TESTSUITE_TREASURER_KEY"

### if you want to add a new test:
### add "newtest" to tests_to_run array (as well as a comment at the head of the file)
### and a new function with exactly that name:
### newtest() { whatever ; }

tests_to_run=(
	"tokentransactions"
	"merkle_vote_plaintext"
	"cspvoting"
	"anonvoting"
	"merkle_vote_encrypted"
	"vocli"
)

# if any arg is passed, treat them as the tests to run, overriding the default list
[ $# != 0 ] && tests_to_run=($@)

initaccounts() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=initaccounts \
		  --oracleKey=$ORACLE_KEY \
		  --treasurerKey=$TREASURER_KEY \
		  --accountKeys=$(echo -n "$ACCOUNT_KEYS" | tr -d ' ','\t' | tr '\n' ',')
}

merkle_vote() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=vtest \
		  --oracleKey=$ORACLE_KEY \
		  --treasurerKey=$TREASURER_KEY \
		  --electionSize=$ELECTION_SIZE \
		  --electionType=$1 \
		  --withWeight=2 \
		  --accountKeys=$(echo $ACCOUNT_KEYS | awk '{print $1}')
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
		  --treasurerKey=$TREASURER_KEY \
		  --electionSize=$ELECTION_SIZE_ANON \
		  --accountKeys=$(echo $ACCOUNT_KEYS | awk '{print $2}')
}

cspvoting() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=cspvoting \
		  --oracleKey=$ORACLE_KEY \
		  --treasurerKey=$TREASURER_KEY \
		  --electionSize=$ELECTION_SIZE \
		  --accountKeys=$(echo $ACCOUNT_KEYS | awk '{print $3}')
}

tokentransactions() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=tokentransactions \
		  --oracleKey=$ORACLE_KEY \
		  --treasurerKey=$TREASURER_KEY \
		  --accountKeys=$(echo $ACCOUNT_KEYS | awk '{print $4}')
}

vocli() {
	docker-compose run test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=vocli \
		  --treasurerKey=$TREASURER_KEY
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
	check_gw_is_up && break || sleep 2
done

# create temp dir
results="/tmp/.vochaintest$RANDOM"
mkdir -p $results

initaccounts

echo "### Test suite ready ###"
for test in ${tests_to_run[@]}; do
	[ $CONCURRENT -eq 1 ] && {
		echo "### Running test $test concurrently with others ###"
		( $test ; echo $? > $results/$test ) &
		sleep 6
	} || {
		echo "### Running test $test ###"
		( $test ; echo $? > $results/$test )
	}
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
		docker-compose logs
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
