#!/bin/bash -xe

NODE_NAME="octopus2"
TEMPLATE_VMID=9003
CI_USER="keights"
CI_PASSWORD="keights"
SSH_KEY_PATH="${HOME}/.ssh/id_rsa.pub"
BASE_IP="192.168.1."
GW="${BASE_IP}1"
VMID_BASE=100


qm set ${TEMPLATE_VMID} --sshkey "${SSH_KEY_PATH}"
qm set ${TEMPLATE_VMID} --ciuser "${CI_USER}" --cipassword "${CI_PASSWORD}"

for COUNT in {1..3}; do
  IP="${BASE_IP}$((COUNT + 2))"
  VMID=$(($VMID_BASE + $COUNT))
  IPCONFIG="ip=${IP}/24,gw=${GW}"
  VM_NAME="keda-vm${COUNT}"

  echo "Creating VM: ${VM_NAME} with IP: ${IP} and VMID: ${VMID}"

  qm clone ${TEMPLATE_VMID} ${VMID} --name "${VM_NAME}" --full true
  qm set ${VMID} --ipconfig0 "${IPCONFIG}"

  qm start ${VMID}

done

