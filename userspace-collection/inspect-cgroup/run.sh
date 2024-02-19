#!/bin/sh

go run inspect-cgroup.go -lower-cpu 20 -upper-cpu 80 -collection-period 3s
