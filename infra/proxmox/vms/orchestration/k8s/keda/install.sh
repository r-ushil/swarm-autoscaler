#!/bin/bash

curl -sS https://webinstall.dev/k9s | bash

snap install helm --classic


helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm install keda kedacore/keda --namespace keda --create-namespace

sleep 3

helm install keda-add-ons-http kedacore/keda-add-ons-http --namespace keda

sleep 3

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install nginx-ingress ingress-nginx/ingress-nginx --namespace default

sleep 3

kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

kubectl get deployment metrics-server -n kube-system -o yaml | \
sed 's/args:/args:\n        - --kubelet-insecure-tls/' | \
kubectl apply -f -

sleep 3

kubectl apply -f ./yml/configmap-delay-php.yml
kubectl apply -f ./yml/nginx-ingress.yml
kubectl apply -f ./yml/apache-deployment.yml
kubectl apply -f ./yml/apache-ingress.yml
kubectl apply -f ./yml/apache-httpscaledobject.yml
