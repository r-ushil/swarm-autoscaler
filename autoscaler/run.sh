#!/bin/sh
cd cgroup_monitoring # go mod replace uses relative paths annoyingly
sudo go run inspect-cgroup.go -lower-cpu 0.1 -upper-cpu 80 -collection-period 3s -iface "wlp60s0"
