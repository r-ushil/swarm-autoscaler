sudo iptables -t nat -I PREROUTING 1 -p tcp --dport 8080 -j DNAT --to-destination 192.168.2.4:8080
sudo iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
sudo iptables -I FORWARD 1 -p tcp --dport 8080 -j ACCEPT
sudo iptables -I FORWARD 1 -m state --state RELATED,ESTABLISHED -j ACCEPT

