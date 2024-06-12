kubectl apply -f ./yml/configmap-delay-php.yml
kubectl apply -f ./yml/nginx-ingress.yml
kubectl apply -f ./yml/apache-deployment.yml
kubectl apply -f ./yml/apache-ingress.yml
kubectl apply -f ./yml/apache-httpscaledobject.yml
