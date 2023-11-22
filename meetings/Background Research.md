
## Autopilot - workload autoscaling at Google
https://dl.acm.org/doi/pdf/10.1145/3342195.3387524

Written in 2020.

Outlines *vertical autoscaling* method used internally - changing individual container limits dynamically. Fine line between under-utilisation and OOM kills.

Uses horizontal autoscaling too, but doesn't go into detail.

Mem limits (cgroups):
- Hard - OOM kill when exceeded
- Soft - can claim more mem, but up to cluster management to commit task-genocide (ones over the limit)

Slack - difference between resource limit and actual usage (**important for defining original limit**)

Uses historic data from previous executions and suggests limits based on this and finely-tuned heuristics (**this has got to be quite resource intensive right?**)

Heuristics (**could be useful for when scheduling autoscaling? or let swarm handle this?**):
- Is the container OOM tolerant?
- Is the container latency tolerant?

Slow decay: Avoid terminating too many tasks at once

CPU/Memory traces of a job over time seems super useful - Prom / Grafana? 

Takeaway: Reactive by using ML, but still seems to poll. Reduces resource utils through optimal vertical/horizontal limits by taking history and predictions into account.


## pHPA: A Proactive Autoscaling Framework for Microservice Chain

https://conferences.sigcomm.org/events/apnet2021/papers/apnet2021-8.pdf

Written in 2021.




## Burst Aware Autoscaler
https://ieeexplore.ieee.org/document/9097467


## Survey of Autoscaling in K8s
https://ieeexplore.ieee.org/document/9829572

Pretty useful stuff tbf.

Also like this:
https://dl.acm.org/doi/pdf/10.1145/3190507

For VMs?:
https://ieeexplore.ieee.org/document/6517993


KEDA!




