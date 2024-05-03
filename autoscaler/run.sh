#!/bin/sh
docker stack deploy -c docker-compose.yml webserver
cd cgroup_monitoring # go mod replace uses relative paths annoyingly
sudo go run inspect-cgroup.go -config "../config.yaml"

