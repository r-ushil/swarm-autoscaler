#!/bin/sh

# do not run this without - converting cgroup_net_activity

go generate ./...
go build .
