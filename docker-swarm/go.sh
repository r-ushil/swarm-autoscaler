#!/bin/bash

sudo systemctl stop firewalld

docker swarm init

# connect other machine to this node

# ensure webserver image on both machines (docker build -t webserver /path/to/webserver

docker service create --name webserver --publish 8080:8080 webserver


ab -n 5000 -c 5 -s 60 http://127.0.0.1:8080/test

# in another window have docker stats open

docker service update --replicas 2 webserver

# run ab again, look at docker stats on both machines


# ----------------------------------------------------------------------------------------------

# using docker compose


docker stack deploy -c docker-compose.yml webserver-jaeger

# to remove

docker stack rm webserver-jaeger


# -------------------------------

#docker stack deploy again

# prometheus endpoint at 9090, use promql to get info

# grafana folder needs to be owned by 472:472
sudo chown -R 472:472 ./monitoring/grafana-data


# in grafana, use http://prometheus:9090


