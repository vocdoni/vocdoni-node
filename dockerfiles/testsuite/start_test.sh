#!/bin/bash
# bash start_test.sh [testname] [testname] [...]
#  (if no argument is passed, run all tests)
#  e2etest_plaintextelection_empty: run poll vote test, with no votes
#  e2etest_plaintextelection: run poll vote test
#  e2etest_encryptedelection: run encrypted vote test
#  e2etest_anonelection: run anonymous vote test
#  e2etest_tokentxs: run token transactions test (no end-user voting at all)
#  e2etest_overwritelection: run overwrite test
#  e2etest_censusizelection: run max census size test
#  e2etest_ballotelection: run ballot test
#  e2etest_lifecyclelection: run lifecycle test
#  e2etest_cspelection: run csp test
#  e2etest_dynamicensuselection: run dynamic census test

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 COMPOSE_INTERACTIVE_NO_CLI=1
[ -n "$GOCOVERDIR" ] && export BUILDARGS="$BUILDARGS -cover" # docker compose build passes this to go 1.20 so that binaries collect code coverage

COMPOSE_CMD=${COMPOSE_CMD:-"docker compose"}
COMPOSE_CMD_RUN="$COMPOSE_CMD run"
[ -n "$NOTTY" ] && COMPOSE_CMD_RUN="$COMPOSE_CMD_RUN -T"

ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-30}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
BUILD=${BUILD:-1}
CLEAN=${CLEAN:-1}
LOGLEVEL=${LOGLEVEL:-debug}
CONCURRENT=${CONCURRENT:-1}
CONCURRENT_CONNECTIONS=${CONCURRENT_CONNECTIONS:-1}
### must be splited by lines
DEFAULT_ACCOUNT_KEYS="73ac72a16ea84dd1f76b62663d2aa380253aec4386935e460dad55d4293a0b11
be9248891bd6c220d013afb4b002f72c8c22cbad9c02003c19729bcbd6962e52
cb595f3fa1a4790dd54c139524a1430fc500f95a02affee6a933fcb88849a48d"
ACCOUNT_KEYS=${ACCOUNT_KEYS:-$DEFAULT_ACCOUNT_KEYS}
GWHOST="http://gateway0:9090/dvote"
APIHOST="http://gateway0:9090/v2"
FAUCET="$APIHOST/open/claim"
TEST_PREFIX="testsuite_test"
RANDOMID="${RANDOM}${RANDOM}"

log() { echo $(date --rfc-3339=s) "$@" ; }

### if you want to add a new test:
### add "newtest" to tests_to_run array (as well as a comment at the head of the file)
### and a new function with exactly that name:
### newtest() { whatever ; }

tests_to_run=(
		"e2etest_raceDuringCommit"
  	"e2etest_plaintextelection_empty"
  	"e2etest_plaintextelection"
  	"e2etest_encryptedelection"
  	"e2etest_anonelection"
  	"e2etest_overwritelection"
  	"e2etest_censusizelection"
  	"e2etest_ballotelection"
  	"e2etest_tokentxs"
  	"e2etest_lifecyclelection"
  	"e2etest_cspelection"
  	"e2etest_dynamicensuselection"
	"test_statesync"
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
	echo "  CONCURRENT_CONNECTIONS=1"
	exit 0
}

# if any arg is passed, treat them as the tests to run, overriding the default list
[ $# != 0 ] && tests_to_run=($@)

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
		  --timeout=10m \
		  $args
}

e2etest_raceDuringCommit() {
  e2etest raceDuringCommit
}

e2etest_plaintextelection() {
	e2etest plaintextelection --votes=100
}

e2etest_plaintextelection_empty() {
	e2etest plaintextelection --votes=0
}

e2etest_encryptedelection() {
	e2etest encryptedelection
}

e2etest_anonelection() {
	e2etest anonelection
	e2etest anonelectionTempSIKs
}

e2etest_hysteresis() {
	e2etest hysteresis
}

e2etest_tokentxs() {
	e2etest tokentxs
}

e2etest_overwritelection() {
  e2etest overwritelection
}

e2etest_censusizelection() {
  e2etest censusizelection
}

e2etest_ballotelection() {
  e2etest ballotelection
}

e2etest_lifecyclelection() {
  e2etest lifecyclelection
}

e2etest_cspelection() {
  e2etest cspelection
}

e2etest_dynamicensuselection() {
  e2etest dynamicensuselection
}

e2etest_ballotelection() {
  e2etest ballotRanked
  e2etest ballotQuadratic
  e2etest ballotRange
  e2etest ballotApproval
}

test_statesync() {
	export VOCDONI_VOCHAIN_STATESYNCFETCHPARAMSFROMAPI=$APIHOST
	$COMPOSE_CMD up gatewaySync -d
	# watch logs for 2 minutes, until catching 'startup complete'. in case of timeout, or panic, or whatever, test will fail
	timeout 120 sh -c "($COMPOSE_CMD logs gatewaySync -f | grep -m 1 'startup complete')"
}

### end tests definition

log "### Starting test suite ###"
[ $BUILD -eq 1 ] && {
	$COMPOSE_CMD build
	$COMPOSE_CMD build test
}
$COMPOSE_CMD up -d seed # start the seed first so the nodes can properly bootstrap
sleep 10
$COMPOSE_CMD up -d miner0 miner1 miner2 miner3 gateway0

check_gw_is_up() {
	date
	$COMPOSE_CMD_RUN test \
		curl -s --fail $APIHOST/chain/info 2>/dev/null
}

log "### Waiting for test suite to be ready ###"
for i in {1..20}; do
	check_gw_is_up && break || sleep 2
done

if [ i == 20 ] ; then
	log "### Timed out waiting! Abort, don't even try running tests ###"
	tests_to_run=()
	GOCOVERDIR=
	RET=30
else
	log "### Test suite ready ###"
fi

# create temp dir
results="/tmp/.vocdoni-test$RANDOM"
mkdir -p $results

for test in ${tests_to_run[@]}; do
	if [ $test == "e2etest_raceDuringCommit" ] || \
	   [ $test == "test_statesync" ] || \
	   [ $CONCURRENT -eq 0 ] ; then
		log "### Running test $test ###"
		( set -o pipefail ; $test | tee $results/$test.stdout ; echo $? > $results/$test.retval )
	else
		log "### Running test $test concurrently with others ###"
		( set -o pipefail ; $test | tee $results/$test.stdout ; echo $? > $results/$test.retval ) &
	fi
	sleep 6
done

log "### Waiting for tests to finish ###"
wait

failed=""
for test in ${tests_to_run[@]} ; do
	RET=$(cat $results/$test.retval)
	if [ "$RET" == "0" ] ; then
		log "### Vochain test $test finished correctly! ###"
	else
		log "### Vochain test $test failed! ###"
		log "### Post run logs ###"
		$COMPOSE_CMD logs --tail 1000
		failed="$test"
		break
	fi
done

if [ -n "$GOCOVERDIR" ] ; then
	log "### Collect all coverage files in $GOCOVERDIR ###"
	rm -rf "$GOCOVERDIR"
	mkdir -p "$GOCOVERDIR"
	$COMPOSE_CMD stop
	$COMPOSE_CMD_RUN --user=`id -u`:`id -g` -v $(pwd):/wd/ gocoverage sh -c "\
		cp -rf /app/run/gocoverage/. /wd/$GOCOVERDIR
		go tool covdata textfmt \
			-i=\$(find /wd/$GOCOVERDIR/ -type d -printf '%p,'| sed 's/,$//') \
			-o=/wd/$GOCOVERDIR/gocoverage-integration.txt
		go tool covdata merge \
			-i=\$(find /wd/$GOCOVERDIR/ -type d -printf '%p,'| sed 's/,$//') \
			-o=/wd/$GOCOVERDIR/
		"
	log "### Coverage data in textfmt left in $GOCOVERDIR/gocoverage-integration.txt ###"
	log "### Coverage data in binary fmt left in $GOCOVERDIR ###"
fi

if $COMPOSE_CMD logs | grep -q "^panic:" ; then
	RET=2
	log "### Panic detected! Look for panic in the logs above for full context, pasting here the 100 lines after panic ###"
	$COMPOSE_CMD logs | grep -A 100 "^panic:"
fi

if $COMPOSE_CMD logs | grep -q "CONSENSUS FAILURE" ; then RET=3 ; log "### CONSENSUS FAILURE detected! Look for it in the logs above ###"; fi

[ $CLEAN -eq 1 ] && {
	log "### Cleaning docker environment ###"
	$COMPOSE_CMD down -v --remove-orphans
}

if [ -n "$failed" ]; then
  log "### Logs from failed test ($test) ###"
  cat $results/$failed.stdout
fi

# remove temp dir
rm -rf $results

if [ "$RET" == "66" ] ; then log "### Race detected! Look for WARNING: DATA RACE in the logs above ###"; fi

exit $RET
