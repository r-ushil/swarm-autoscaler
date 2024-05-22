#!/bin/bash

cd ../autoscaler
go build -o ../build/swarm-autoscaler autoscaler.go

cd ../build
docker build -t rushpate/swarm-autoscaler:latest .
docker push rushpate/swarm-autoscaler 
