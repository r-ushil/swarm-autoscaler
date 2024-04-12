#!/bin/bash

set -xe

IMAGE_NAME="debian-12-generic-amd64.qcow2"
VOLUME_NAME="local-lvm"
VIRTUAL_MACHINE_ID="9000"
TEMPLATE_NAME="debian-12-template"
TMP_CORES="2"
TMP_MEMORY="8192"  # 8GB of RAM
ROOT_PASSWD="playground-turtle"


wget https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-arm64.qcow2

apt update && apt install libguestfs-tools -y

virt-customize -a ${IMAGE_NAME} --install qemu-guest-agent
virt-customize -a ${IMAGE_NAME} --root-password password:${ROOT_PASSWD}

qm create ${VIRTUAL_MACHINE_ID} --name ${TEMPLATE_NAME} --memory ${TMP_MEMORY} --cores ${TMP_CORES} --net0 virtio,bridge=vmbr1

qm importdisk ${VIRTUAL_MACHINE_ID} ${IMAGE_NAME} ${VOLUME_NAME} --format qcow2

qm set ${VIRTUAL_MACHINE_ID} --scsihw virtio-scsi-pci --scsi0 ${VOLUME_NAME}:vm-${VIRTUAL_MACHINE_ID}-disk-0
qm set ${VIRTUAL_MACHINE_ID} --boot c --bootdisk scsi0
qm set ${VIRTUAL_MACHINE_ID} --ide2 ${VOLUME_NAME}:cloudinit
qm set ${VIRTUAL_MACHINE_ID} --serial0 socket --vga serial0
qm set ${VIRTUAL_MACHINE_ID} --agent enabled=1

qm template ${VIRTUAL_MACHINE_ID}

echo "Debian 12 Cloud-Init template named ${TEMPLATE_NAME} with ID ${VIRTUAL_MACHINE_ID} is ready."

