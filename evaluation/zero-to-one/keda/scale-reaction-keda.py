import requests
import time
import subprocess
import re
from datetime import datetime

url = "http://apache.local:30080/delay.php"
log_file_path = "/root/autoscaler_monitor.log"
ssh_node = "node1"
monitor_script_path = "/root/monitor_autoscaler_logs.sh"

def measure_scale_up_latency(url, log_file_path, ssh_node, monitor_script_path):
    # Start the monitor_autoscaler_logs.sh script on node1
    ssh_command = f"ssh {ssh_node} 'nohup {monitor_script_path} > /dev/null 2>&1 &'"
    subprocess.run(ssh_command, shell=True)

    # Make the initial request to start the scale-up process
    try:
        start_time = datetime.utcnow()
        response = requests.get(url)
        print(f"Request made at: {start_time.isoformat()}Z")

        if response.status_code == 200:
            print("Service responded successfully.")
        else:
            print(f"Service responded with status code: {response.status_code}")
    except requests.exceptions.RequestException as e:
        print(f"Request failed: {e}")
        return

    # Wait for the monitoring script to detect the scaling event
    while True:
        try:
            ssh_command = f"ssh {ssh_node} 'cat {log_file_path}'"
            result = subprocess.run(ssh_command, shell=True, capture_output=True, text=True)
            log_output = result.stdout.strip()

            if log_output:
                log_timestamp = datetime.strptime(log_output, '%Y-%m-%dT%H:%M:%S.%fZ')
                print(f"Autoscaler recognized scaling need at: {log_timestamp.isoformat()}Z")

                # Calculate the delta
                delta = (log_timestamp - start_time).total_seconds()
                print(f"Recognition latency: {delta:.8f} seconds")
                break
        except Exception as e:
            print(f"Error reading log file: {e}")
        
        # Sleep for a short interval before checking again
        time.sleep(0.1)

measure_scale_up_latency(url, log_file_path, ssh_node, monitor_script_path)

