kubectl apply -f ./yml-httpd/configmap-delay-php.yml
kubectl apply -f ./yml-httpd/nginx-ingress.yml
kubectl apply -f ./yml-httpd/httpd-deployment.yml
kubectl apply -f ./yml-httpd/httpd-ingress.yml
kubectl apply -f ./yml-httpd/httpd-httpscaledobject.yml
