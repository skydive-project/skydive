#!/bin/bash

set -v

# Install requirements
sudo yum install -y https://www.rdoproject.org/repos/rdo-release.rpm
sudo yum -y install make openvswitch unzip docker
sudo service docker start
sudo service openvswitch start
sudo ovs-appctl -t ovsdb-server ovsdb-server/add-remote ptcp:6400

rpm -qi openvswitch
