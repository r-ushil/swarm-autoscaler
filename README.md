# swarm-autoscaler
A Docker Swarm autoscaler that monitors service load using eBPF.


## Playground

Service - Likely needs socket mounting, eBPF / admin capabilities.
  - Is it going to have access to the cgroup directory?
  - What if we autoscale the autoscaler? Need some sort of quorum logic to stop conflicting scaling.

Might be able to either use bpftrace or write from scratch (no need to reinvent the wheel if that does the job)

1. Write service that can connect to docker socket and autoscale based on label (true or false)
2. Gather metrics (CPU, mem and network) from this service (can use Docker Stats API for now) and then autoscale based on label
3. Use eBPF to gather metrics
4. Quorum testing
5. Avg. metric over 15 minutes


Future dev:
  - write the hook as a kernel function / add to bcc or libbpf
  - (maybe) have as a docker network plugin?

