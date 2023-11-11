import socket
from flask import Flask, jsonify, request
from opentelemetry import trace
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource

app = Flask(__name__)


resource = Resource(attributes={"service.name": "webserver"})
trace.set_tracer_provider(TracerProvider(resource=resource))


exporter = OTLPSpanExporter(
    endpoint="http://jaeger:4317",
    insecure=True,
)

trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(exporter)
)

FlaskInstrumentor().instrument_app(app)
tracer = trace.get_tracer(__name__)


@app.before_request
def before_req_otel():
    tracer = trace.get_tracer(__name__)
    current_span = trace.get_current_span()
    current_span.set_attribute("instance_id", socket.gethostname())

@app.route('/test')
def test():
    ip_addr = request.remote_addr
    return jsonify(message=ip_addr)

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=8080, threaded=True)

