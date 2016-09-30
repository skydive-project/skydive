#!/bin/bash

set -v

# Install requirements
sudo yum install -y https://www.rdoproject.org/repos/rdo-release.rpm
sudo yum -y install make openvswitch unzip docker libpcap-devel etcd libxml2-devel
sudo service docker start
sudo service openvswitch start
sudo service etcd start

rpm -qi openvswitch
