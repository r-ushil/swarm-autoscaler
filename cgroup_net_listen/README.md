
scale-to-zero.go pauses the container, loads an eBPF program to detect network traffic for the cgroup and unpauses when this is detected.



./build.sh generates the `bpf2go` files (all prepended with bpf\_bpfe\*) for the Cilium library.

To try this out, run

`sudo ./scale-to-zero $CONTAINER_ID`

To build from source, run go get, and convert scale-to-zero to a standalone program (change ScaleToZero(containerID) to main() that takes containerID as arg) and run `./build.sh`.
