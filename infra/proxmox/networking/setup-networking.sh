#!/bin/bash

# Define paths to the new configuration files (assuming this script is run in the same directory as the config files)
NEW_INTERFACES="./interfaces"
NEW_NFTABLES_CONF="./nftables.conf"

# Enable IP Forwarding
sed -i '/^#net.ipv4.ip_forward=1/s/^#//' /etc/sysctl.conf
grep -qxF 'net.ipv4.ip_forward=1' /etc/sysctl.conf || echo 'net.ipv4.ip_forward=1' | tee -a /etc/sysctl.conf

sysctl -p


# Backup current configurations
#echo "Backing up current configurations..."
#cp /etc/network/interfaces /etc/network/interfaces.backup.$(date +%F-%H-%M-%S)
#cp /etc/nftables.conf /etc/nftables.conf.backup.$(date +%F-%H-%M-%S)

# Update /etc/network/interfaces
if [ -f "${NEW_INTERFACES}" ]; then
    echo "Updating /etc/network/interfaces..."
    cp "${NEW_INTERFACES}" /etc/network/interfaces
    systemctl restart networking
else
    echo "New interfaces file not found. Skipping..."
fi

# Update /etc/nftables.conf
if [ -f "${NEW_NFTABLES_CONF}" ]; then
    echo "Updating /etc/nftables.conf..."
    cp "${NEW_NFTABLES_CONF}" /etc/nftables.conf
    systemctl restart nftables
else
    echo "New nftables.conf file not found. Skipping..."
fi


echo "Networking setup has been updated."

