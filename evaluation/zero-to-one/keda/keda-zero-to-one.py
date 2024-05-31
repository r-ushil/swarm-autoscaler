#!/usr/bin/python3

import requests
import time

url = "http://apache.local:30080/delay.php"

def measure_scale_up_latency(url):
  start_time = time.time()
  while True:
    try:
      response = requests.get(url)
      if response.status_code == 200:
        end_time = time.time()
        total_latency = end_time - start_time
        print(f"{total_latency:.8f}")
        break
      else:
        print(response.text)
    except requests.exceptions.RequestException as e: 
      pass

measure_scale_up_latency(url)
