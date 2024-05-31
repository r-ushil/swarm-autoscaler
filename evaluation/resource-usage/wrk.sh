wrk -t1 -c20 -d650s -H "Host: apache.default.example.com" http://192.168.1.3:30080/

#wrk -t1 -c20 -d650s http://apache.local:30080/delay.php

#wrk -t1 -c20 -d650s -H "Connection: Close" http://192.168.2.4:8080


