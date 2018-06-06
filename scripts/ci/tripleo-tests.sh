#!/bin/bash

set -e

curl -Lo skydive-latest https://github.com/skydive-project/skydive-binaries/raw/jenkins-builds/skydive-latest
chmod +x skydive-latest
sudo mv skydive-latest /usr/bin/skydive

. ~/stackrc
CTLIP=$( openstack server show overcloud-controller-0 -f json | jq -r .addresses | cut -d '=' -f 2 )
AGENTS_COUNT=$( SKYDIVE_ANALYZERS=$CTLIP:8082 skydive client status --username skydive --password skydive | jq '.Agents | length' )
if [ $AGENTS_COUNT -lt 2 ]; then
	echo Expected agent count not found
	SKYDIVE_ANALYZERS=$CTLIP:8082 skydive client status --username skydive --password skydive
	exit 1
fi

. ~/overcloudrc
curl -o /tmp/cirros.img http://download.cirros-cloud.net/0.4.0/cirros-0.4.0-x86_64-disk.img
openstack image create --public --file /tmp/cirros.img --disk-format raw cirros
openstack flavor create --vcpus 1 --ram 64 --disk 1 --public tiny

NETID=$( openstack network create private -f json | jq -r .id )
openstack subnet create --subnet-range 10.0.0.0/24 --network $NETID private

openstack server create --image cirros --flavor tiny --nic net-id=$NETID vm1

for i in {1..30}; do

	INTF_COUNT=$( SKYDIVE_ANALYZERS=$CTLIP:8082 \
		skydive client query "G.V().HasKey('Neutron').Count()" --username skydive --password skydive )
	if [ $INTF_COUNT -eq 5 ]; then
		exit 0
	fi

	sleep 5

done
