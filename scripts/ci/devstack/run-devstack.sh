#!/bin/bash

set -v

[ -z "$SKYDIVE_PATH" ] && SKYDIVE_PATH=`pwd`

sudo yum -y install git iproute net-tools
git clone https://git.openstack.org/openstack-dev/devstack devstack.git
cd devstack.git

# temp until an upstream fix
sed -i -e 's/wait_for_service 30/wait_for_service 120/' pkg/elasticsearch.sh

export PATH=$PATH:/usr/sbin
host_ip_iface=${host_ip_iface:-$(ip -f inet route | awk '/default/ {print $5}' | head -1)}
host_ips=$(LC_ALL=C ip -f inet addr show ${host_ip_iface} | sed /temporary/d |awk /inet'/ {split($2,parts,"/");  print parts[1]}')
echo "host_ip_iface=$host_ip_iface"
echo "host_ips=$host_ips"

# preset password for mariadb, workaround for
# https://bugs.launchpad.net/devstack/+bug/1706125
sudo yum -y install mariadb-server
sudo systemctl start mariadb

mysql -u root <<-EOF
UPDATE mysql.user SET Password=PASSWORD('RTh56v9_33') WHERE User='root';
FLUSH PRIVILEGES;
EOF

cat << EOF > local.conf
[[local|localrc]]

DATABASE_PASSWORD=RTh56v9_33
RABBIT_PASSWORD=RTh56v9_33
SERVICE_PASSWORD=RTh56v9_33
SERVICE_TOKEN=RTh56v9_33
ADMIN_PASSWORD=RTh56v9_33

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
enable_plugin skydive file://$SKYDIVE_PATH devstack
enable_service skydive-analyzer skydive-agent

SKYDIVE_ANALYZER_LISTEN=0.0.0.0:8082
SKYDIVE_AGENT_LISTEN=0.0.0.0:8081

SKYDIVE_FLOWS_STORAGE=elasticsearch
SKYDIVE_GRAPH_STORAGE=elasticsearch

EOF

./stack.sh
