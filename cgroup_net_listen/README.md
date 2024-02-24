
cgroup_net_listen.go recevies the container id for a paused container (called from `../userspace-collection/scale/scale.go`), loads an eBPF program to detect network traffic for the cgroup and returns when this is detected.



./build.sh generates the `bpf2go` files (all prepended with bpf\_bpfe\*) for the Cilium library.

To test this functionality out (and automatically pause and unpause the container), run

`sudo ./compiled-test $CONTAINER_ID`
