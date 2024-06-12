import requests
import time
import subprocess
import re
from datetime import datetime

url = "http://192.168.1.4:30080"
host_header = "apache.default.example.com"
log_script_path = "./get-autoscaler-log.sh"
ssh_node = "node1"

def measure_scale_up_latency(url, host_header, log_script_path, ssh_node):
    start_time = datetime.utcnow()
    print(f"Request made at: {start_time.isoformat()}Z")
    
    # Make the initial request to start the scale-up process
    try:
        response = requests.get(url, headers={"Host": host_header})
        if response.status_code == 200:
            print("Service responded successfully.")
        else:
            print(f"Service responded with status code: {response.status_code}")
    except requests.exceptions.RequestException as e:
        print(f"Request failed: {e}")
        return

    # Wait for a few seconds to allow the autoscaler to react
    time.sleep(10)

    # Run the get-autoscaler-logs.sh script via SSH
    ssh_command = f"ssh {ssh_node} -tt {log_script_path}"
    result = subprocess.run(ssh_command, shell=True, capture_output=True, text=True)
    
    # Extract the timestamp from the log output
    log_output = result.stdout
    print("Log output:\n", log_output)
    
    # Match the timestamp in the log output using regex
    match = re.search(r'"timestamp":"([^"]+)"', log_output)
    if match:
        log_timestamp_str = match.group(1)
        log_timestamp_str = log_timestamp_str[:26]  # Truncate to microseconds
        log_timestamp = datetime.fromisoformat(log_timestamp_str)
        print(f"Autoscaler reacted at: {log_timestamp.isoformat()}Z")
        
        # Calculate the delta
        delta = (log_timestamp - start_time).total_seconds()
        print(f"Scale-up reaction time: {delta:.8f} seconds")
    else:
        print("Timestamp not found in the log output.")

measure_scale_up_latency(url, host_header, log_script_path, ssh_node)

