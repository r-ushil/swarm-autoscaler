#!/bin/bash

pveam download local debian-12-standard_12.2-1_amd64.tar.zst

pct create 100 local:vztmpl/debian-12-standard_12.2-1_amd64.tar.zst --storage local-lvm \
	--hostname toolbox \
	--cores 2 \
	--memory 4096 \
	--net0 name=eth0,bridge=vmbr1,ip=192.168.1.2/24,gw=192.168.1.1 \
	--net1 name=eth1,bridge=vmbr2,ip=192.168.2.2/24,gw=192.168.2.1 \
	--password toolbox
