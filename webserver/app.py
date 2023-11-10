from flask import Flask
import time

app = Flask(__name__)

# Global variable to store the time of the last request
last_request_time = time.time()

@app.route('/test')
def test():
    global last_request_time
    
    # Calculate time since the last request
    current_time = time.time()
    time_difference = current_time - last_request_time
    last_request_time = current_time

    # Adjust workload based on request frequency
    # If requests are coming in faster, 'time_difference' will be small
    # If they're coming in slower, 'time_difference' will be larger
    workload_factor = max(1, int(1000 * time_difference))
    
    # CPU-intensive task proportional to request frequency
    for _ in range(workload_factor):
        _ = sum([x*x for x in range(10)])

    return 'Done!', 200

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080, threaded=True)

