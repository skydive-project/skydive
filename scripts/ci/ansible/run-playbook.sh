#!/bin/bash


# install only : jenkins slaves
ansible-playbook -i inventory.yml -t slave -e ansible_python_interpreter=/usr/bin/python3 -e jenkins_public_hostname=ci.skydive.network -e jenkins_admin_password=password deploy.yml
