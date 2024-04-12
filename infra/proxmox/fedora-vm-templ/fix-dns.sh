#!/bin/bash

# Set DNS servers
nmcli con mod "cloud-init eth0" ipv4.dns "155.198.142.8,155.198.142.7"

# Set the search domain
nmcli con mod "cloud-init eth0" ipv4.dns-search "doc.ic.ac.uk"

# Ensure the connection does not use DNS servers from DHCP
nmcli con mod "cloud-init eth0" ipv4.ignore-auto-dns yes

# Bring down and then up the connection
nmcli con down "cloud-init eth0"
nmcli con up "cloud-init eth0"

# Restart NetworkManager service
sudo systemctl restart NetworkManager

yum update -y
