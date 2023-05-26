#!/bin/bash
# bash start_test.sh [testname] [testname] [...]
#  (if no argument is passed, run all tests)
#  legacy_cspvoting: run (rpc_client) csp vote test
#  e2etest_plaintextelection_empty: run poll vote test, with no votes
#  e2etest_plaintextelection: run poll vote test
#  e2etest_encryptedelection: run encrypted vote test
#  e2etest_anonelection: run anonymous vote test
#  e2etest_tokentxs: run token transactions test (no end-user voting at all)
#  e2etest_overwritelection: run overwrite test
#  e2etest_cenususizelection: run max census size test
#
export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 COMPOSE_INTERACTIVE_NO_CLI=1
[ -n "$GOCOVERDIR" ] && export BUILDARGS="-cover" # docker-compose build passes this to go 1.20 so that binaries collect code coverage

COMPOSE_CMD=${COMPOSE_CMD:-"docker-compose"}
COMPOSE_CMD_RUN="$COMPOSE_CMD run"
[ -n "$NOTTY" ] && COMPOSE_CMD_RUN="$COMPOSE_CMD_RUN -T"

ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-30}
ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
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
FAUCET="$APIHOST/faucet/dev/"
. env.treasurerkey # contains var VOCDONI_SIGNINGKEY, import into current env
TREASURER_KEY="$VOCDONI_SIGNINGKEY"
[ -n "$TESTSUITE_TREASURER_KEY" ] && TREASURER_KEY="$TESTSUITE_TREASURER_KEY"
TEST_PREFIX="testsuite_test"
RANDOMID="${RANDOM}${RANDOM}"

log() { echo $(date --rfc-3339=s) "$@" ; }

### if you want to add a new test:
### add "newtest" to tests_to_run array (as well as a comment at the head of the file)
### and a new function with exactly that name:
### newtest() { whatever ; }

tests_to_run=(
	"legacy_cspvoting"
	"e2etest_plaintextelection_empty"
	"e2etest_plaintextelection"
	"e2etest_encryptedelection"
	"e2etest_anonelection"
	"e2etest_overwritelection"
	"e2etest_cenususizelection"
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
	echo "  CONCURRENT_CONNECTIONS=1"
	exit 0
}

# if any arg is passed, treat them as the tests to run, overriding the default list
[ $# != 0 ] && tests_to_run=($@)

legacy_cspvoting() {
	$COMPOSE_CMD_RUN --name ${TEST_PREFIX}_${FUNCNAME[0]}_${RANDOMID} test timeout 300 \
		./vochaintest --gwHost $GWHOST \
		  --logLevel=$LOGLEVEL \
		  --operation=cspvoting \
		  --treasurerKey=$TREASURER_KEY \
		  --electionSize=$ELECTION_SIZE \
		  --accountKeys=$(echo $ACCOUNT_KEYS | awk '{print $3}')
}

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
}

e2etest_tokentxs() {
	e2etest tokentxs
}

e2etest_overwritelection () {
  e2etest overwritelection
}

e2etest_cenususizelection () {
  e2etest censusizelection
}

### end tests definition

# useful for debugging bash flow
# docker-compose() { echo "# would do: docker-compose $@" ; sleep 0.2 ;}

log "### Starting test suite ###"
$COMPOSE_CMD build
$COMPOSE_CMD up -d seed # start the seed first so the nodes can properly bootstrap
sleep 10
$COMPOSE_CMD up -d

check_gw_is_up() {
	date
	$COMPOSE_CMD_RUN test \
		curl -s --fail $APIHOST/chain/info 2>/dev/null
}

log "### Waiting for test suite to be ready ###"
for i in {1..20}; do
	check_gw_is_up && break || sleep 2
done
log "### Test suite ready ###"

# create temp dir
results="/tmp/.vochaintest$RANDOM"
mkdir -p $results

for test in ${tests_to_run[@]}; do
	if [ $test == "tokentransactions" ] || [ $CONCURRENT -eq 0 ] ; then
		log "### Running test $test ###"
		( set -o pipefail ; $test | tee $results/$test.stdout ; echo $? > $results/$test.retval )
	else
		log "### Running test $test concurrently with others ###"
		( set -o pipefail ; $test | tee $results/$test.stdout ; echo $? > $results/$test.retval ) &
	fi
	sleep 6
done

log "### Waiting for tests to finish ###"

wait_jobs() {
	while [ -n "$(jobs)" ]; do
		for file in $results/*.retval ; do
			test=$(basename $file .retval)
			RET=$(cat $file)
			if [ "$RET" == "0" ] ; then
				log "### Vochain test $test finished correctly! ###"
				rm $file
			else
				log "### Vochain test $test failed! (aborting all tests) ###"
				$COMPOSE_CMD down
				log "### Post run logs ###"
				$COMPOSE_CMD logs --tail 1000
				failed="$test"
				return
			fi
		done
		sleep 1
	done
}

wait_jobs

if [ -n "$GOCOVERDIR" ] ; then
	log "### Collect all coverage files in $GOCOVERDIR ###"
	mkdir -p $GOCOVERDIR
	$COMPOSE_CMD down
	$COMPOSE_CMD_RUN --user=`id -u`:`id -g` -v $(pwd):/wd/ gocoverage sh -c "\
		cp -rf /app/run/gocoverage/. /wd/$GOCOVERDIR
		go tool covdata textfmt \
			-i=\$(find /wd/$GOCOVERDIR/ -type d -printf '%p,'| sed 's/,$//') \
			-o=/wd/$GOCOVERDIR/covdata.txt
		"
	log "### Coverage data in textfmt left in $GOCOVERDIR/covdata.txt ###"
fi

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

exit $RET
