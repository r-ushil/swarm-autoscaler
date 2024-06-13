#!/usr/bin/python3

import requests
import time

url = "http://192.168.2.5:8080"
retry_delay = 2

def measure_scale_up_latency(url, retry_delay):
  start_time = time.time()
  while True:
    try:
      response = requests.get(url)
      if response.status_code == 200:
        end_time = time.time()
        total_latency = end_time - start_time
        print(f"{total_latency:.8f}")
        break
    except requests.exceptions.RequestException as e: 
      pass

measure_scale_up_latency(url, retry_delay)
