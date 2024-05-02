sudo iptables -t nat -D PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 192.168.2.4:8080
sudo iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
sudo iptables -D FORWARD -p tcp --dport 8080 -j ACCEPT
sudo iptables -D FORWARD -m state --state RELATED,ESTABLISHED -j ACCEPT

