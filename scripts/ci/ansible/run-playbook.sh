#!/bin/bash


# install only : jenkins slaves
dir="$(dirname "$0")"
ANSIBLE_ROLES_PATH="~/.ansible/roles:/usr/share/ansible/roles:/etc/ansible/roles:$dir/../../../contrib/ansible/roles" ansible-playbook -i inventory.yml -t slave --skip-tags vagrant -e ansible_python_interpreter=/usr/bin/python3 -e jenkins_public_hostname=ci.skydive.network -e jenkins_admin_password=password deploy.yml
