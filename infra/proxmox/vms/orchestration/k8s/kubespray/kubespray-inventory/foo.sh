declare -a IPS=(keights@192.168.1.3 keights@192.168.1.4 keights@192.168.1.5) 
CONFIG_FILE=inventory/mycluster/hosts.yaml python3 contrib/inventory_builder/inventory.py ${IPS[@]}


