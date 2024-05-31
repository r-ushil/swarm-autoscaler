#!/bin/bash

curl -sS https://webinstall.dev/k9s | bash

snap install helm --classic

# Knative
kubectl apply -f https://github.com/knative/operator/releases/download/knative-v1.14.2/operator.yaml

sleep 3
kubectl apply -f ./yml/knative-serving.yml

sleep 3
kubectl patch configmap/config-domain \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"example.com":""}}'

kubectl patch svc kourier -n knative-serving -p '{"spec": {"type": "NodePort", "ports": [{"port": 80, "nodePort": 30080}]}}'

kubectl apply -f ./yml/configmap-delay-php.yml

kubectl apply -f ./yml/apache-service.yml

kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

kubectl get deployment metrics-server -n kube-system -o yaml | \
sed 's/args:/args:\n        - --kubelet-insecure-tls/' | \
kubectl apply -f -

# Test with
# curl -H "Host: apache.default.example.com" http://$NODE_IP:30080
