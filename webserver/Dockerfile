FROM python:3.8-slim

WORKDIR /app

COPY app.py /app/

RUN pip3 install Flask opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-flask

EXPOSE 8080

CMD ["python", "app.py"]

