#!/bin/sh

go run inspect-cgroup.go -lower-cpu 5 -upper-cpu 80 -collection-period 3s
