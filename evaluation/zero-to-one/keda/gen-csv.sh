#!/bin/bash

output_file="keda_zero_to_one.csv"
duration=600  # 10 minutes in seconds
interval=60   # Interval in seconds

echo "time,second_number" > $output_file
end_time=$((SECONDS + duration))
iteration=1

while [ $SECONDS -lt $end_time ]; do
  start_time=$(date +%s)
  current_time=$(date +"%Y-%m-%d %H:%M:%S")
  output=$(python3 keda-zero-to-one.py)

  # Extract the second number from the output
  second_number=$(echo "$output" | tail -n 1)

  echo "$current_time,$second_number" >> $output_file
  echo "Iteration: $iteration"

  # Calculate elapsed time and sleep for the remaining time to maintain the interval
  end_execution_time=$(date +%s)
  elapsed_time=$((end_execution_time - start_time))
  sleep_time=$((interval - elapsed_time))

  if [ $sleep_time -gt 0 ]; then
    sleep $sleep_time
  fi

  iteration=$((iteration + 1))
done

