#!/bin/bash

set -v

git checkout -b software-factory
SKYDIVE_PATH=`pwd`

sudo yum -y install git iproute net-tools
git clone https://git.openstack.org/openstack-dev/devstack devstack.git
cd devstack.git

export PATH=$PATH:/usr/sbin
host_ip_iface=${host_ip_iface:-$(ip -f inet route | awk '/default/ {print $5}' | head -1)}
host_ips=$(LC_ALL=C ip -f inet addr show ${host_ip_iface} | sed /temporary/d |awk /inet'/ {split($2,parts,"/");  print parts[1]}')
echo "host_ip_iface=$host_ip_iface"
echo "host_ips=$host_ips"

cat << EOF > local.conf
[[local|localrc]]

DATABASE_PASSWORD=password
RABBIT_PASSWORD=password
SERVICE_PASSWORD=password
SERVICE_TOKEN=password
ADMIN_PASSWORD=password

HOST_IP=$host_ips
HOST_IP_IFACE=eth0

# Disable glance
disable_service g-api
disable_service g-reg

# Disable nova
disable_service n-api
disable_service n-crt
disable_service n-cpu
disable_service n-net
disable_service n-cond
disable_service n-sch
disable_service n-cauth

# Enable Neutron
enable_service q-svc
enable_service q-dhcp
enable_service q-meta
enable_service q-agt
enable_service q-l3

# Disable tempest
disable_service tempest

# Disable cinder
disable_service c-sch
disable_service c-api
disable_service c-vol

# Do not use horizon
disable_service horizon

ENABLE_ISOLATED_METADATA=True

# Skydive
enable_plugin skydive file://$SKYDIVE_PATH software-factory
enable_service skydive-analyzer skydive-agent

SKYDIVE_ANALYZER_LISTEN=0.0.0.0:8082
SKYDIVE_AGENT_LISTEN=0.0.0.0:8081
EOF

./stack.sh

set -e

source openrc admin admin
export PATH=$PATH:/opt/go/bin:/opt/stack/go/bin:/opt/stack/protoc/bin
export GOROOT=/opt/go
export GOPATH=/opt/stack/go
export GO_VERSION=1.6
cd /opt/stack/go/src/github.com/skydive-project/skydive/
SKYDIVE_AGENT_ANALYZERS=localhost:8082 make test.functionals TAGS="neutron" VERBOSE=true TIMEOUT=2m TEST_PATTERN=Neutron
