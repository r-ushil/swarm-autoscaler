FROM python:3.8-slim

WORKDIR /app

COPY app.py /app/

RUN pip3 install Flask prometheus_flask_exporter

EXPOSE 8080

CMD ["python", "app.py"]

