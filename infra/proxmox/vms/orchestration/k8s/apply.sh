#!/bin/bash

kubectl apply -f ./yml/configmap-delay-php.yml
kubectl apply -f ./yml/apache-deployment.yml
kubectl apply -f ./yml/apache-hpa.yml
