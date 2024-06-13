#!/usr/bin/python3

import requests
import time
from datetime import datetime

url = "http://192.168.2.5:8080"
retry_delay = 2

def measure_scale_up_latency(url, retry_delay):
  start_time = datetime.utcnow()
  while True:
    try:
      response = requests.get(url)
      if response.status_code == 200:
        print(start_time)
        break
    except requests.exceptions.RequestException as e: 
      pass

measure_scale_up_latency(url, retry_delay)
