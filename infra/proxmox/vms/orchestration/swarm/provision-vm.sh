#!/bin/bash -xe

NODE_NAME="octopus2"
TEMPLATE_VMID=9002
CI_USER="swarm"
CI_PASSWORD="swarm"
SSH_KEY_PATH="${HOME}/.ssh/id_rsa.pub"
BASE_IP="192.168.2."
GW="${BASE_IP}1"
VMID_BASE=400


qm set ${TEMPLATE_VMID} --sshkey "${SSH_KEY_PATH}"
qm set ${TEMPLATE_VMID} --ciuser "${CI_USER}" --cipassword "${CI_PASSWORD}"

# provision monitoring VM

IP="${BASE_IP}3"
IPCONFIG="ip=${IP}/24,gw=${GW}"
VM_NAME="monitoring"

echo "Creating VM: ${VM_NAME} with IP: ${IP} and VMID: ${VMID_BASE}"
qm clone ${TEMPLATE_VMID} ${VMID_BASE} --name "${VM_NAME}" --full true
qm set ${VMID_BASE} --ipconfig0 "${IPCONFIG}"
qm start ${VMID_BASE}

# provision swarm VMs
for COUNT in {1..3}; do
  IP="${BASE_IP}$((COUNT + 3))"
  VMID=$(($VMID_BASE + $COUNT))
  IPCONFIG="ip=${IP}/24,gw=${GW}"
  VM_NAME="swarm-vm${COUNT}"

  echo "Creating VM: ${VM_NAME} with IP: ${IP} and VMID: ${VMID}"

  qm clone ${TEMPLATE_VMID} ${VMID} --name "${VM_NAME}" --full true
  qm set ${VMID} --ipconfig0 "${IPCONFIG}"

  qm start ${VMID}

done

