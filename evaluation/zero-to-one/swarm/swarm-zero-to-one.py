import requests
import time

url = "http://192.168.2.4:8080"
retry_delay = 2

def measure_scale_up_latency(url, retry_delay):
  request_count = 0
  start_time = time.time()
  response_time_list = []
  while True:
    request_count += 1
    attempt_start_time = time.time()
    try:
      response = requests.get(url)
      if response.status_code == 200:
        print(response.text)
        end_time = time.time()
        total_latency = end_time - start_time
        print(f"Service responded in {total_latency:.8f} seconds after {request_count} requests.")
        break
      else:
        print(f"Received status code {response.status_code}. Retrying...")
    except requests.exceptions.RequestException as e: 
      pass
      #print(f"Request failed: {e}. Retrying...")
    attempt_end_time = time.time()
    response_time_list.append(attempt_end_time - attempt_start_time)
    #time.sleep(retry_delay)

  program_end_time = time.time()
  program_exec_time = program_end_time - start_time
  average_request_interval = sum(response_time_list) / len(response_time_list) if response_time_list else 0
  print(f"Total requests sent: {request_count}")
  print(f"Average time between requests: {average_request_interval:.8f} seconds")
  print(f"Program exec time: {program_exec_time:.8f} seconds")

measure_scale_up_latency(url, retry_delay)
