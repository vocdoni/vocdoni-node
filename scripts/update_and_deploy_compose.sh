#!/bin/bash
BRANCH=${BRANCH:-master}
CMD=${CMD:-vocdoninode}
NAME="$CMD-$BRANCH"

export COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1

[ ! -d dockerfiles/$CMD ] && {
	echo "dockerfiles/$CMD does not exist"
	echo "please execute this script from repository root: bash scripts/update_and_deploy_compose.sh"
	exit 1
}

check_git() { # 0=no | 1=yes
	[ -n "$FORCE" ] && echo "Force is enabled" && return 1
	git fetch origin
	local is_updated=$(git log HEAD..origin/$BRANCH --oneline | wc -c) # 0=yes
	[ $is_updated -gt 0 ] && git pull origin $BRANCH --force && return 1
	return 0
}

deploy() {
	echo "Updating and deploying container"
	cd dockerfiles/$CMD
	docker-compose build
	docker-compose up -d
	exit $?
}

check_git || deploy

echo "nothing to do, use FORCE=1 $0 if you want to force the update"
