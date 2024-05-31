#!/bin/bash

curl -sS https://webinstall.dev/k9s | bash

snap install helm --classic


helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm install keda kedacore/keda --namespace keda --create-namespace

sleep 3

helm install keda-add-ons-http kedacore/keda-add-ons-http --namespace keda \
  --set interceptor.waitTimeout=5s \
  --set interceptor.tcpConnectTimeout=1s \
  --set interceptor.keepAlive=30s \
  --set interceptor.responseHeaderTimeout=5s \
  --set interceptor.idleConnTimeout=30s \
  --set interceptor.tlsHandshakeTimeout=10s \
  --set interceptor.expectContinueTimeout=1s

#helm install keda-add-ons-http kedacore/keda-add-ons-http --namespace keda

sleep 3

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install nginx-ingress ingress-nginx/ingress-nginx --namespace default

sleep 3

kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

kubectl get deployment metrics-server -n kube-system -o yaml | \
sed 's/args:/args:\n        - --kubelet-insecure-tls/' | \
kubectl apply -f -

sleep 30

kubectl apply -f ./yml-httpd/configmap-delay-php.yml
kubectl apply -f ./yml-httpd/nginx-ingress.yml
kubectl apply -f ./yml-httpd/httpd-deployment.yml
kubectl apply -f ./yml-httpd/httpd-ingress.yml
kubectl apply -f ./yml-httpd/httpd-httpscaledobject.yml
