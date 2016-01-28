#!/bin/bash

# create/delete a test topology
# syntax:
#   ./multinode.sh start <ns ip/mask> [tunnel endpoint ip]
#   ./multinode.sh stop
#
# options:
#   tunnel endpoint ip can be used in a mutlinodes environnement


function start() {
	set -x

	sudo ovs-vsctl add-br br-int

	sudo ip netns add vm1
	sudo ip link add vm1-eth0 type veth peer name eth0 netns vm1
	sudo ip link set vm1-eth0 up

	sudo ip netns exec vm1 ip link set eth0 up
	sudo ip netns exec vm1 ip address add $1 dev eth0

	sudo ovs-vsctl add-port br-int vm1-eth0

        if [ ! -z "$2" ]; then
		sudo ovs-vsctl add-br br-tun
		sudo ovs-vsctl add-port br-int patch-int -- set interface patch-int type=patch
		sudo ovs-vsctl add-port br-tun patch-tun -- set interface patch-tun type=patch
		sudo ovs-vsctl set interface patch-int option:peer=patch-tun
		sudo ovs-vsctl set interface patch-tun option:peer=patch-int
		sudo ovs-vsctl add-port br-tun mn-gre0 -- set interface mn-gre0 type=gre options:remote_ip=$2
	fi
}

function stop() {
	set -x

	sudo ovs-vsctl del-br br-tun
	sudo ovs-vsctl del-br br-int
	sudo ip link del vm1-eth0
	sudo ip netns del vm1	
}


if [ "$1" == "start" ]; then
	if [ -z "$2" ]; then
		echo "Usage: $0 start <ns ip/mask> [tunnel endpoint ip]"
		exit 1
	fi

	start $2 $3
else
	stop
fi
