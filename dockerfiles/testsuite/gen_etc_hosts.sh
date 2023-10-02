#!/bin/bash

# Backup the original /etc/hosts file

# Filter out the Docker entries from the original /etc/hosts to a temporary file
grep -v "# Docker Entry" /etc/hosts > ./hosts

# Fetch running container details and append to the temporary file
docker ps --format '{{.Names}}' | while read container_name; do
    container_ip=$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $container_name)
    name="$(echo $container_name | cut -d- -f2)"
    if [ ! -z "$container_ip" ]; then
        echo "$container_ip $name # Docker Entry" >> ./hosts
	echo "$name => $container_ip"
    fi
done

echo "
=> Replace the original /etc/hosts with the updated one
sudo mv ./hosts /etc/hosts"
