FROM ubuntu:latest

WORKDIR /autoscaler

COPY ./swarm-autoscaler ./config.yaml ./

RUN chmod +x swarm-autoscaler

CMD ["./swarm-autoscaler", "-config", "./config.yaml"]

