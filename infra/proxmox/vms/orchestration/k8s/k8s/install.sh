#!/bin/bash

curl -sS https://webinstall.dev/k9s | bash

snap install helm --classic


kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

kubectl get deployment metrics-server -n kube-system -o yaml | \
sed 's/args:/args:\n        - --kubelet-insecure-tls/' | \
kubectl apply -f -

sleep 20

./apply.sh
