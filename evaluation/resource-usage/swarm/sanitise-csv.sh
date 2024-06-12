#!/bin/bash

# Input and output files
SWARM_INPUT_FILE="swarm_metrics_idle.csv"
SWARM_OUTPUT_FILE="swarm_metrics_idle_sanitised.csv"

# Function to sanitise swarm metrics
sanitise_swarm_metrics() {
  input_file=$1
  output_file=$2

  # CSV header
  echo "timestamp,namespace,pod,cpu_usage,memory_usage" > $output_file

  # Read the input file and sanitise data
  tail -n +2 $input_file | while IFS=',' read -r timestamp host container_id cpu_usage memory_usage; do
    # Remove '%' from CPU usage and convert to millicores
    cpu_usage_percent=$(echo $cpu_usage | sed 's/%//')
    cpu_usage_millicores=$(echo "$cpu_usage_percent * 10" | bc)

    # Extract memory usage (before the slash) and convert to MiB
    memory_usage=$(echo $memory_usage | awk '{print $1}')
    if [[ $memory_usage == *GiB ]]; then
      memory_usage=$(echo $memory_usage | sed 's/GiB/*1024/' | bc)
    elif [[ $memory_usage == *MiB ]]; then
      memory_usage=$(echo $memory_usage | sed 's/MiB//')
    elif [[ $memory_usage == *KiB ]]; then
      memory_usage=$(echo $memory_usage | sed 's/KiB//')
      memory_usage=$(echo "scale=2; $memory_usage/1024" | bc)
    fi

    # Convert the memory usage to a whole number if needed
    memory_usage=$(printf "%.0fMi" $memory_usage)

    # Assume namespace and pod names based on the provided data
    namespace="swarm"
    pod="${host}-${container_id}"

    echo "$timestamp,$namespace,$pod,${cpu_usage_millicores}m,$memory_usage" >> $output_file
  done
}

# Sanitise the swarm metrics
sanitise_swarm_metrics $SWARM_INPUT_FILE $SWARM_OUTPUT_FILE

echo "Swarm metrics sanitised and saved to $SWARM_OUTPUT_FILE"

