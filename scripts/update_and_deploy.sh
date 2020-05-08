#!/bin/bash
BRANCH=${BRANCH:-release-0.3}
CMD=${CMD:-dvotenode}
NAME="$CMD-$BRANCH"

[ ! -d dockerfiles/$CMD ] && {
  echo "dockerfiles/$CMD does not exist"
  echo "please execute this script from repository root: bash scripts/update_and_deploy.sh"
  exit 1
}

check_git() { # 0=no | 1=yes
	[ -n "$FORCE" ] && echo "Force is enabled" && return 1
	git fetch origin
	local is_updated=$(git log HEAD..origin/$BRANCH --oneline | wc -c) # 0=yes
	[ $is_updated -gt 0 ] && git pull origin $BRANCH --force && return 1
	return 0
}

check_git || {
 echo "Updating and deploying container"
 for f in `docker container ls | grep $NAME | awk '{print $1}' | grep -v CONTAINER`; do docker container stop $f; done
 docker container prune -f
 NAME="$NAME" EXTRA_OPTS="$EXTRA_OPTS" dockerfiles/$CMD/dockerlaunch.sh
 exit $?
}
echo "nothing to do, use FORCE=1 $0 if you want to force the update"
