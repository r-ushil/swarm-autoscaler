#!/bin/bash

service_name="apache"
namespace="default"
pid_file="/root/monitor_knative_scale.pid"
echo $$ > $pid_file

prev_replicas=$(kubectl get pods -l serving.knative.dev/service=$service_name --field-selector=status.phase=Running --no-headers | wc -l)

while true; do
  current_replicas=$(kubectl get pods -l serving.knative.dev/service=$service_name --field-selector=status.phase=Running --no-headers | wc -l)
  if [ "$current_replicas" != "$prev_replicas" ]; then
    scale_time=$(date +"%Y-%m-%dT%H:%M:%S.%6NZ")
    if [ "$current_replicas" -gt "$prev_replicas" ]; then
      echo "Scale Up Time: $scale_time"
      echo $scale_time > /root/scale-up-time.log
    fi
    break
  fi
  sleep 1
done

# Cleanup PID file
rm -f $pid_file

