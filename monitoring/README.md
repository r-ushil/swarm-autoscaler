Need to use this config scraper (type A, dns sd) with overlay network for Swarm.



# PromQL

Average latency over 5m

```
sum(rate(flask_request_latency_seconds_sum{job="webserver"}[5m])) by (hostname) / sum(rate(flask_request_latency_seconds_count{job="webserver"}[5m])) by (hostname)
```

95% in histogram

```
histogram_quantile(0.95, sum(rate(flask_request_latency_seconds_bucket{job="webserver"}[5m])) by (le, hostname))
```



