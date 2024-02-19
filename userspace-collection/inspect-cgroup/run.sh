#!/bin/sh


go run inspect-cgroup.go -lower-mm 20 -upper-mm 55 -collection-period 3s
