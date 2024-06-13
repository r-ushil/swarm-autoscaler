#!/bin/bash

OUTPUT_FILE="swarm_metrics.csv"
duration=600  # 5 minutes in seconds
interval=30   # Interval in seconds

# CSV header
echo "timestamp,host,container_id,cpu_usage,memory_usage" > $OUTPUT_FILE

# Function to collect metrics
collect_metrics() {
  host=$1
  ssh $host "docker stats --no-stream --format '{{.Container}},{{.CPUPerc}},{{.MemUsage}}' autoscaler-swarm-autoscaler-1" | while read -r line; do
    timestamp=$(date +%Y-%m-%dT%H:%M:%S)
    echo "$timestamp,$host,$line" >> $OUTPUT_FILE
  done
}

end_time=$((SECONDS + duration))
iteration=1

while [ $SECONDS -lt $end_time ]; do
  start_time=$(date +%s)
  echo "Collecting metrics - Iteration $iteration"
  collect_metrics "swarm-vm1"
  collect_metrics "swarm-vm2"
  collect_metrics "swarm-vm3"

  # Calculate elapsed time and sleep for the remaining time to maintain the interval
  end_execution_time=$(date +%s)
  elapsed_time=$((end_execution_time - start_time))
  sleep_time=$((interval - elapsed_time))

  if [ $sleep_time -gt 0 ]; then
    sleep $sleep_time
  fi

  iteration=$((iteration + 1))
done

