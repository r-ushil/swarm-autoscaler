#!/bin/bash

OUTPUT_FILE="knative_metrics.csv"
duration=600  # 5 minutes in seconds
interval=30   # Interval in seconds

# CSV header
echo "timestamp,namespace,pod,cpu_usage,memory_usage" > $OUTPUT_FILE

# Function to collect metrics
collect_metrics() {
  namespace=$1
  kubectl top pods -n $namespace --no-headers | while read -r pod cpu mem; do
    timestamp=$(date +%Y-%m-%dT%H:%M:%S)
    echo "$timestamp,$namespace,$pod,$cpu,$mem" >> $OUTPUT_FILE
  done
}

end_time=$((SECONDS + duration))
iteration=1

while [ $SECONDS -lt $end_time ]; do
  start_time=$(date +%s)
  echo "Collecting metrics - Iteration $iteration"
  collect_metrics "knative-serving"
  collect_metrics "knative-operator"

  # Calculate elapsed time and sleep for the remaining time to maintain the interval
  end_execution_time=$(date +%s)
  elapsed_time=$((end_execution_time - start_time))
  sleep_time=$((interval - elapsed_time))

  if [ $sleep_time -gt 0 ]; then
    sleep $sleep_time
  fi

  iteration=$((iteration + 1))
done

