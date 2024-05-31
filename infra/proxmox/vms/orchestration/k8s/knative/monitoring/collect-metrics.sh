#!/bin/bash

OUTPUT_FILE="knative_metrics.csv"

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

# Run the metric collection every 30 seconds for 5 minutes
for i in {1..10}
do
  echo "Collecting metrics - Iteration $i"
  collect_metrics "knative-serving"
  collect_metrics "knative-operator"
  sleep 30
done
