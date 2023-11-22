import os
from flask import Flask, jsonify, request
from prometheus_flask_exporter import PrometheusMetrics

app = Flask(__name__)

metrics = PrometheusMetrics(app, group_by='endpoint')
metrics.info('app_info', 'Webserver', version='0.0.2')

# Get hostname - make sure this is a callable for labels
hostname_callable = lambda: os.getenv('HOSTNAME', 'localhost')

request_latency = metrics.histogram(
    'flask_request_latency_seconds',
    'Latency of HTTP requests in seconds',
    labels={'hostname': hostname_callable}
)

@app.route('/test')
@request_latency
def test():
    ip_addr = request.remote_addr
    return jsonify(message=ip_addr)

if __name__ == '__main__':
    app.run(debug=False, host='0.0.0.0', port=8080)
