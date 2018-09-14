#!/bin/bash

set -e

USERNAME=skydive
PASSWORD=secret

. ~/stackrc
CTLIP=$( openstack server show overcloud-controller-0 -f json | jq -r .addresses | cut -d '=' -f 2 )

. ~/overcloudrc
NETID=$( openstack network create private -f json | jq -r .id )
openstack subnet create --subnet-range 10.0.0.0/24 --network $NETID private

sleep 60

bash ~/skydive-test.sh -a $CTLIP:8082 -u $USERNAME -p $PASSWORD -e 2 -c -i -o -n
