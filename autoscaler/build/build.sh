#!/bin/bash

cd ../cgroup_monitoring
go build -o ../build/swarm-autoscaler inspect-cgroup.go

cd ../build
docker build -t rushpate/swarm-autoscaler:latest .
#docker push rushpate/swarm-autoscaler 
