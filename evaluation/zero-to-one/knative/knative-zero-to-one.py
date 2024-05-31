#!/usr/bin/python3

import requests
import time

url = "http://192.168.1.3:30080"
host_header = "apache.default.example.com"

def measure_scale_up_latency(url, host_header):
  start_time = time.time()
  while True:
    try:
      response = requests.get(url, headers={"Host": host_header})
      if response.status_code == 200:
        end_time = time.time()
        total_latency = end_time - start_time
        print(f"{total_latency:.8f}")
        break
    except requests.exceptions.RequestException as e: 
      pass

measure_scale_up_latency(url, host_header)
