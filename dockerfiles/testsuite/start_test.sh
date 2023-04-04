#!/bin/bash
# bash start_test.sh [testname] [testname] [...]
#  (if no argument is passed, run all tests)
#  legacy_cspvoting: run (rpc_client) csp vote test
#  e2etest_plaintextelection: run poll vote test
#  e2etest_encryptedelection: run encrypted vote test
#  e2etest_anonelection: run anonymous vote test
#  e2etest_tokentxs: run token transactions test (no end-user voting at all)

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 COMPOSE_INTERACTIVE_NO_CLI=1

COMPOSE_CMD=${COMPOSE_CMD:-"docker-compose"}
COMPOSE_CMD_RUN="$COMPOSE_CMD run"
[ -n "$NOTTY" ] && COMPOSE_CMD_RUN="$COMPOSE_CMD_RUN -T"

ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-30}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
CLEAN=${CLEAN:-1}
LOGLEVEL=${LOGLEVEL:-debug}
CONCURRENT=${CONCURRENT:-0}
### must be splited by lines
DEFAULT_ACCOUNT_KEYS="73ac72a16ea84dd1f76b62663d2aa380253aec4386935e460dad55d4293a0b11
be9248891bd6c220d013afb4b002f72c8c22cbad9c02003c19729bcbd6962e52
cb595f3fa1a4790dd54c139524a1430fc500f95a02affee6a933fcb88849a48d"
ACCOUNT_KEYS=${ACCOUNT_KEYS:-$DEFAULT_ACCOUNT_KEYS}
GWHOST="http://gateway0:9090/dvote"
APIHOST="http://gateway0:9090/v2"
FAUCET="$APIHOST/faucet/dev/"
. env.oracle0key # contains var VOCDONI_SIGNINGKEY, import into current env
ORACLE_KEY="$VOCDONI_SIGNINGKEY"
[ -n "$TESTSUITE_ORACLE_KEY" ] && ORACLE_KEY="$TESTSUITE_ORACLE_KEY"
. env.treasurerkey # contains var VOCDONI_SIGNINGKEY, import into current env
TREASURER_KEY="$VOCDONI_SIGNINGKEY"
[ -n "$TESTSUITE_TREASURER_KEY" ] && TREASURER_KEY="$TESTSUITE_TREASURER_KEY"
TEST_PREFIX="testsuite_test"
RANDOMID="${RANDOM}${RANDOM}"

### if you want to add a new test:
### add "newtest" to tests_to_run array (as well as a comment at the head of the file)
### and a new function with exactly that name:
### newtest() { whatever ; }

tests_to_run=(
	"legacy_cspvoting"
	"e2etest_plaintextelection"
	"e2etest_encryptedelection"
	"e2etest_anonelection"
	"e2etest_tokentxs"
)

# print help
[ "$1" == "-h" -o "$1" == "--help" ] && {
	echo "$0 <test_to_run>"
	echo "available tests: ${tests_to_run[@]}"
	echo "env vars:"
	echo "  CLEAN=1"
	echo "  ELECTION_SIZE=30"
	echo "  ELECTION_SIZE_ANON=8"
	echo "  CONCURRENT=0"
	echo "  LOGLEVEL=debug"
	echo "  GWHOST=http://gateway0:9090/dvote"
	exit 0
}

# if any arg is passed, treat them as the tests to run, overriding the default list
[ $# != 0 ] && tests_to_run=($@)

legacy_cspvoting() {
	$COMPOSE_CMD_RUN --name ${TEST_PREFIX}_${FUNCNAME[0]}_${RANDOMID} test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=cspvoting \
		  --oracleKey=$ORACLE_KEY \
		  --treasurerKey=$TREASURER_KEY \
		  --electionSize=$ELECTION_SIZE \
		  --accountKeys=$(echo $ACCOUNT_KEYS | awk '{print $3}')
}

e2etest() {
	$COMPOSE_CMD_RUN --name ${TEST_PREFIX}_${FUNCNAME[0]}-${1}_${RANDOMID} test timeout 300 \
		./end2endtest --host $APIHOST --faucet=$FAUCET \
		  --logLevel=$LOGLEVEL \
		  --operation=$1
}

e2etest_plaintextelection() {
	e2etest plaintextelection
}

e2etest_encryptedelection() {
	e2etest encryptedelection
}

e2etest_anonelection() {
	e2etest anonelection
}

e2etest_tokentxs() {
	e2etest tokentxs
}

### end tests definition

# useful for debugging bash flow
# docker-compose() { echo "# would do: docker-compose $@" ; sleep 0.2 ;}

echo "### Starting test suite ###"
$COMPOSE_CMD build
$COMPOSE_CMD up -d

check_gw_is_up() {
	$COMPOSE_CMD_RUN test \
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

echo "### Test suite ready ###"
for test in ${tests_to_run[@]}; do
	if [ $test == "tokentransactions" ] || [ $CONCURRENT -eq 0 ] ; then
		echo "### Running test $test ###"
		( set -o pipefail ; $test | tee $results/$test.stdout ; echo $? > $results/$test.retval )
	else
		echo "### Running test $test concurrently with others ###"
		( set -o pipefail ; $test | tee $results/$test.stdout ; echo $? > $results/$test.retval ) &
	fi
	sleep 6
done

echo "### Waiting for tests to finish ###"
wait

failed=""
for test in ${tests_to_run[@]} ; do
	RET=$(cat $results/$test.retval)
	if [ "$RET" == "0" ] ; then
		echo "Vochain test $test finished correctly!"
	else
		echo "Vochain test $test failed!"
		echo "### Post run logs ###"
		$COMPOSE_CMD logs --tail 1000
		failed="$test"
		break
	fi
done

[ $CLEAN -eq 1 ] && {
	echo "### Cleaning docker environment ###"
	$COMPOSE_CMD down -v --remove-orphans
}

if [ -n "$failed" ]; then
  echo "### Logs from failed test ($test) ###"
  cat $results/$failed.stdout
fi

# remove temp dir
rm -rf $results

exit $RET
