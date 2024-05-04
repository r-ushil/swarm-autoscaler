#!/bin/sh
docker stack deploy -c httpd-stack.yml webserver
cd cgroup_monitoring # go mod replace uses relative paths annoyingly
sudo go run inspect-cgroup.go -config "../build/config.yaml"

