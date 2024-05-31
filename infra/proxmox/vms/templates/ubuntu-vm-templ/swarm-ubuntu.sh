#!/bin/bash

set -xe

IMAGE_NAME="ubuntu-24.04-server-cloudimg-amd64.img"
VOLUME_NAME="local-lvm"
VIRTUAL_MACHINE_ID="9004"
TEMPLATE_NAME="ubuntu-24.04-template-k8s"
TMP_CORES="2"
TMP_MEMORY="8192"  # 8GB of RAM
DISK_SIZE="8G"
ROOT_PASSWD="playground-turtle"

apt update && apt install libguestfs-tools -y

wget https://cloud-images.ubuntu.com/releases/24.04/release/${IMAGE_NAME}

# Update sshd_config to permit root login
virt-customize -a ${IMAGE_NAME} --edit '/etc/ssh/sshd_config:s/^#PermitRootLogin prohibit-password/PermitRootLogin yes/'
virt-customize -a ${IMAGE_NAME} --edit '/etc/ssh/sshd_config:s/^PasswordAuthentication no/PasswordAuthentication yes/'

# Update cloud.cfg to allow root login and password authentication
virt-customize -a ${IMAGE_NAME} --run-command 'sed -i "s/disable_root: true/disable_root: false/" /etc/cloud/cloud.cfg'
virt-customize -a ${IMAGE_NAME} --run-command 'echo -e "manage_etc_hosts: false" >> /etc/cloud/cloud.cfg'

# Install qemu-guest-agent and set the root password
virt-customize -a ${IMAGE_NAME} --install qemu-guest-agent
virt-customize -a ${IMAGE_NAME} --root-password password:${ROOT_PASSWD}

# Create the VM template in Proxmox
qm create ${VIRTUAL_MACHINE_ID} --name ${TEMPLATE_NAME} --memory ${TMP_MEMORY} --cores ${TMP_CORES} --net0 virtio,bridge=vmbr2

qm importdisk ${VIRTUAL_MACHINE_ID} ${IMAGE_NAME} ${VOLUME_NAME} --format qcow2
qm set ${VIRTUAL_MACHINE_ID} --scsihw virtio-scsi-pci --scsi0 ${VOLUME_NAME}:vm-${VIRTUAL_MACHINE_ID}-disk-0
qm resize ${VIRTUAL_MACHINE_ID} scsi0 ${DISK_SIZE}

qm set ${VIRTUAL_MACHINE_ID} --boot c --bootdisk scsi0
qm set ${VIRTUAL_MACHINE_ID} --ide2 ${VOLUME_NAME}:cloudinit
qm set ${VIRTUAL_MACHINE_ID} --serial0 socket --vga serial0
qm set ${VIRTUAL_MACHINE_ID} --agent enabled=1

qm template ${VIRTUAL_MACHINE_ID}

echo "Ubuntu 24.04 Cloud-Init template named ${TEMPLATE_NAME} with ID ${VIRTUAL_MACHINE_ID} is ready."

