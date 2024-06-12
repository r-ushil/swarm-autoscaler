#!/usr/bin/python3
import subprocess
import time
import datetime
import csv
import os

def get_timestamp():
    return datetime.datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%S.%fZ')

# Function to execute a command via SSH and return the output
def ssh_exec_command(host, command):
    ssh_command = f'ssh {host} "{command}"'
    result = os.popen(ssh_command).read().strip()
    return result

# Function to start the load using wrk
def start_wrk():
    wrk_command = ["wrk", "-t1", "-c5", "-d650s", "http://192.168.1.3:30080/delay.php"]
    process = subprocess.Popen(wrk_command)
    return process

def constant_wrk():
    wrk_command = ["wrk", "-t1", "-c1", "-d650s", "http://192.168.1.3:30080/delay.php"]
    process = subprocess.Popen(wrk_command)
    return process


# Function to measure the scale-up latency for one iteration
def measure_latency(iteration, ssh_host):
    wrk2 = None
    try:

        # SSH into the Kubernetes node and start the monitoring script
        monitor_command = "nohup /root/monitor_k8s_scale.sh > /dev/null 2>&1 &"
        ssh_exec_command(ssh_host, monitor_command)
        print("Starting monitoring")

        # Record the start time
        scale_up_start_time = get_timestamp()
        # Start an additional load
        wrk2 = start_wrk()
        print(f"Scale Up Start Time: {scale_up_start_time}")

        time.sleep(20)

        # Wait for scale-up to happen
        scale_up_time = None
        while True:
            result = ssh_exec_command(ssh_host, "test -f /root/scale-up-time.log && cat /root/scale-up-time.log")
            if result:
                scale_up_time = result
                break
            time.sleep(1)

        # Calculate the delta
        scale_up_start_epoch = datetime.datetime.strptime(scale_up_start_time, '%Y-%m-%dT%H:%M:%S.%fZ').timestamp()
        scale_up_epoch = datetime.datetime.strptime(scale_up_time, '%Y-%m-%dT%H:%M:%S.%fZ').timestamp()
        delta = scale_up_epoch - scale_up_start_epoch

        # Print and write the delta to a CSV file
        print(f"Iteration {iteration}: Scale-Up Latency: {delta} seconds")
        with open("scale_up_latency.csv", "a", newline='') as csvfile:
            csvwriter = csv.writer(csvfile)
            csvwriter.writerow([iteration, scale_up_start_time, scale_up_time, delta])
    finally:
        if wrk2:
            wrk2.terminate()
            wrk2.wait()

        # Ensure the monitoring script is terminated
        monitor_pid = ssh_exec_command(ssh_host, "cat /root/monitor_knative_scale.pid")
        if monitor_pid:
            ssh_exec_command(ssh_host, f"kill {monitor_pid}")

if __name__ == "__main__":
    ssh_host = "node1"

    # Start the constant load
    constant_wrk = constant_wrk()
    time.sleep(30)

    try:
        for iteration in range(1, 11):
            measure_latency(iteration, ssh_host)
            time.sleep(60)
    finally:
        # Ensure the constant load wrk process is terminated
        constant_wrk.terminate()
        constant_wrk.wait()

