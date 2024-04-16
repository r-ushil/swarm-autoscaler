#!/bin/sh
cd cgroup_monitoring # go mod replace uses relative paths annoyingly
sudo ${GOPATH}/bin/dlv debug inspect-cgroup.go -- -lower-cpu 5 -upper-cpu 80 -collection-period 3s -iface "wlp60s0"
