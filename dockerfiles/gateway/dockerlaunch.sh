#!/bin/bash

# INFO
IMAGE_TAG="vocdoni/gateway"
API_PORT="${API_PORT:-9090}"
IN_MEMORY="${IN_MEMORY:-false}"

echo "Using image '$IMAGE_TAG:latest'\n"

docker build -t $IMAGE_TAG -f dockerfile.gateway . || {
	echo "ERROR: docker image cannot be created, exiting..."
	exit 2
}

# CHECK IF ALREADY RUNNING
COUNT="$(docker ps -a | grep $IMAGE_TAG | wc -l)"

[ "$COUNT" != "0" ] && {
	echo -e "\nWARNING: A container with tag $IMAGE_TAG is already running\n"
	docker ps -a | grep $IMAGE_TAG
	echo -e "\nSkipping 'docker run'"
	exit 2
}

ENVFILE=""
[ -f dockerfiles/gateway/env ] && ENVFILE="dockerfiles/gateway/env"
[ -f env ] && ENVFILE="env"
[ -n "$ENVFILE" ] && echo "using ENV FILE $ENVFILE" 

[ ! -d run ] && mkdir run

[ "$IN_MEMORY" == "true" ] && EXTRA_OPTS="$EXTRA_OPTS --volume-driver memfs"

# RUN DOCKER
docker run --name `echo $IMAGE_TAG-$RANDOM | tr "/" "-"` -d \
	-p 4001:4001 -p 5001:5001 -p $API_PORT:9090 -p 30303:30303 -p 9096:9096 -p 31000:31000 -p 26656:26656 \
	-v $PWD/run:/app/run $EXTRA_OPTS \
	`[ -n "$ENVFILE" ] && echo -n "--env-file $ENVFILE"` \
	$IMAGE_TAG
