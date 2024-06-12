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
def do_wrk():
    wrk_command = ["wrk", "-t1", "-c10", "-d650s", "-H", "Connection: Close", "http://192.168.2.4:8080"]
    process = subprocess.Popen(wrk_command)
    return process

# Function to measure the scale-up latency for one iteration
def measure_latency(iteration):
    try:
        # Start the initial load
        wrk1 = do_wrk()

        time.sleep(15)
        # SSH into the Swarm cluster machine and start the monitoring script
        ssh_host = "swarm-vm1"
        monitor_command = "nohup /root/monitor_scale.sh > /dev/null 2>&1 &"
        ssh_exec_command(ssh_host, monitor_command)
        print("Starting monitoring")
        time.sleep(5)

        # Record the start time
        load_start_time = get_timestamp()
        wrk2 = do_wrk()
        print(f"Start Time: {load_start_time}")

        # Wait for 10 seconds
        time.sleep(10)

        scale_up_time = None
        while True:
            if ssh_exec_command(ssh_host, "test -f /root/scale-up-time.log && cat /root/scale-up-time.log"):
                scale_up_time = ssh_exec_command(ssh_host, "cat /root/scale-up-time.log")
                break
            time.sleep(1)

        # Calculate the delta
        load_start_epoch = datetime.datetime.strptime(load_start_time, '%Y-%m-%dT%H:%M:%S.%fZ').timestamp()
        scale_up_epoch = datetime.datetime.strptime(scale_up_time, '%Y-%m-%dT%H:%M:%S.%fZ').timestamp()
        delta = scale_up_epoch - load_start_epoch

        # Print and write the delta to a CSV file
        print(f"Iteration {iteration}: Scale-Up Latency: {delta} seconds")
        with open("scale_up_latency.csv", "a", newline='') as csvfile:
            csvwriter = csv.writer(csvfile)
            csvwriter.writerow([iteration, load_start_time, scale_up_time, delta])
    finally:
        wrk1.kill()
        wrk2.kill()

if __name__ == "__main__":
    for iteration in range(1, 11):
        measure_latency(iteration)
        time.sleep(30)

