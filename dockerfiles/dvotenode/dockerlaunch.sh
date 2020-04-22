#!/bin/bash
IMAGE_TAG="vocdoni/dvotenode"
API_PORT="${API_PORT:-9090}"
IN_MEMORY="${IN_MEMORY:-false}"
PORTS="${PORTS:-4001 4171 5001 30303 9096 31000 26656 26657}"
NAME="${NAME:-$IMAGE_TAG-$RANDOM}"

echo "using image $IMAGE_TAG:latest"

docker build -t $IMAGE_TAG --target dvotenode . || {
	echo "Error: docker image cannot be created, exiting..."
	exit 2
}

# CHECK IF ALREADY RUNNING
COUNT="$(docker ps -a | grep $NAME | wc -l)"

[ "$COUNT" != "0" ] && {
	echo "Error: a container with tag $NAME is already running: $(docker ps -a | grep $NAME)"
	exit 2
}

ENVFILE=""

[ -f env.local ] && ENVFILE="env.local" || {
[ -f dockerfiles/dvotenode/env.local ] && ENVFILE="dockerfiles/dvotenode/env.local" || {
[ -f env ] && ENVFILE="env" || {
[ -f dockerfiles/dvotenode/env ] && ENVFILE="dockerfiles/dvotenode/env"
};};}

[ -n "$ENVFILE" ] && echo "using ENV FILE $ENVFILE" || echo "Warning, no ENV file found!"

[ ! -d run ] && mkdir run

[ "$IN_MEMORY" == "true" ] && EXTRA_OPTS="$EXTRA_OPTS --volume-driver memfs"

echo "mapped ports: $API_PORT $PORTS"

# RUN DOCKER
docker run --name `echo $NAME | tr "/" "-"` -d \
	`for p in $API_PORT $PORTS; do echo -n "-p $p:$p "; done` \
	-v $PWD/run:/app/run -v $PWD/misc:/app/misc $EXTRA_OPTS \
	`[ -n "$ENVFILE" ] && echo -n "--env-file $ENVFILE"` \
	$IMAGE_TAG
