#!/bin/bash -xe
ansible-galaxy collection install community.docker

ansible-playbook -i inventory.ini setup.yml
