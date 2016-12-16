#!/bin/bash

set -v

# Install requirements
sudo yum install -y https://www.rdoproject.org/repos/rdo-release.rpm
sudo yum -y install make openvswitch unzip docker libpcap-devel etcd libxml2-devel jq
sudo service docker start
sudo service openvswitch start
sudo service etcd start

rpm -qi openvswitch

mkdir ${HOME}/protoc
pushd ${HOME}/protoc
wget https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip
unzip protoc-3.1.0-linux-x86_64.zip
popd
export PATH=${HOME}/protoc/bin:${PATH}
