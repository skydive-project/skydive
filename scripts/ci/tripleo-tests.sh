#!/bin/bash

set -e

USERNAME=skydive
PASSWORD=secret

. ~/stackrc
CTLIP=$( openstack server show overcloud-controller-0 -f json | jq -r .addresses | cut -d '=' -f 2 )
AGENTS_COUNT=$( SKYDIVE_ANALYZERS=$CTLIP:8082 /home/stack/skydive client status --username $USERNAME --password $PASSWORD | jq '.Agents | length' )
if [ $AGENTS_COUNT -lt 2 ]; then
	echo Expected agent count not found
	SKYDIVE_ANALYZERS=$CTLIP:8082 /home/stack/skydive client status --username $USERNAME --password $PASSWORD
	exit 1
fi

. ~/overcloudrc
curl -o /tmp/cirros.img http://download.cirros-cloud.net/0.4.0/cirros-0.4.0-x86_64-disk.img
openstack image create --public --file /tmp/cirros.img --disk-format raw cirros
openstack flavor create --vcpus 1 --ram 64 --disk 1 --public tiny

NETID=$( openstack network create private -f json | jq -r .id )
openstack subnet create --subnet-range 10.0.0.0/24 --network $NETID private

# FIX(safchain) currenlty openstack volume client is broken, so can't boot a VM
# will restore it once fixed
#openstack server create --image cirros --flavor tiny --nic net-id=$NETID vm1

# for now, one ns with dhcp agent, on container for the dhcp agent, one ns for the network created and
# one tap for the network as well.
EXPECTED=4

for i in {1..30}; do

	INTF_COUNT=$( SKYDIVE_ANALYZERS=$CTLIP:8082 \
		/home/stack/skydive client query "G.V().Has('Manager', 'neutron').Count()" --username $USERNAME --password $PASSWORD )
	if [ $INTF_COUNT -eq $EXPECTED ]; then
		exit 0
	fi

	sleep 5
done

exit 1
