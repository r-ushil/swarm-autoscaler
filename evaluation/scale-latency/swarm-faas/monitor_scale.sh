#!/bin/bash

rm -f /root/scale-up-time.log
service_name="ws_webserver"
pid_file="/root/monitor_scale.pid"
echo $$ > $pid_file

prev_replicas=$(docker service ls --filter name=$service_name -q | xargs docker service inspect --format '{{.Spec.Mode.Replicated.Replicas}}')

while true; do
  current_replicas=$(docker service ls --filter name=$service_name -q | xargs docker service inspect --format '{{.Spec.Mode.Replicated.Replicas}}')
  if [ "$current_replicas" != "$prev_replicas" ]; then
    scale_time=$(date +"%Y-%m-%dT%H:%M:%S.%6NZ")
    echo "Scale Up Time: $scale_time"
    echo $scale_time > /root/scale-up-time.log
    break
  fi
  sleep 1
done

# Cleanup PID file
rm -f $pid_file

