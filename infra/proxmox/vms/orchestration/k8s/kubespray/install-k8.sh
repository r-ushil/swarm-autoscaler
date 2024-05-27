docker run --rm \
  --mount type=bind,source="$(pwd)/kubespray-inventory",target=/inventory \
  --mount type=bind,source="${HOME}/.ssh/id_rsa",target=/root/.ssh/id_rsa \
  quay.io/kubespray/kubespray:v2.25.0 \
  ansible-playbook -i /inventory/inventory.ini --private-key /root/.ssh/id_rsa cluster.yml

