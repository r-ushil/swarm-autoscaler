#!/bin/bash

cd ../conc_req_monitoring
rm bpf_*
go generate ./...

cd ../autoscaler
go build -o ../build/swarm-autoscaler autoscaler.go

cd ../build
docker build -t rushpate/swarm-autoscaler:latest .
#docker push rushpate/swarm-autoscaler 
