#!/bin/bash

# create/delete a test topology
# syntax:
#   ./simple.sh start <ns ip/mask> [ns2 ip/mask]
#   ./simple.sh stop
#
# options:
#   ns2 ip/mask: create a second ns


function start() {
	set -x

	sudo ovs-vsctl add-br br-int

	sudo ip netns add vm1

	sudo brctl addbr br-vm1
	sudo ip link set br-vm1 up

	sudo ip link add ovs-vm1-eth0 type veth peer name br-vm1-eth0
	sudo ip link set ovs-vm1-eth0 up
	sudo ip link set br-vm1-eth0 up

	sudo brctl addif br-vm1 br-vm1-eth0
	sudo ovs-vsctl add-port br-int ovs-vm1-eth0

	sudo ip link add vm1-eth0 type veth peer name eth0 netns vm1
	sudo brctl addif br-vm1 vm1-eth0

	sudo ip link set vm1-eth0 up

	sudo ip netns exec vm1 ip link set eth0 up
	sudo ip netns exec vm1 ip address add $1 dev eth0

	if [ ! -z "$2" ]; then
		sudo ip netns add vm2

		sudo brctl addbr br-vm2
		sudo ip link set br-vm2 up

		sudo ip link add ovs-vm2-eth0 type veth peer name br-vm2-eth0
		sudo ip link set ovs-vm2-eth0 up
		sudo ip link set br-vm2-eth0 up

		sudo brctl addif br-vm2 br-vm2-eth0
		sudo ovs-vsctl add-port br-int ovs-vm2-eth0

		sudo ip link add vm2-eth0 type veth peer name eth0 netns vm2
		sudo brctl addif br-vm2 vm2-eth0

		sudo ip link set vm2-eth0 up

		sudo ip netns exec vm2 ip link set eth0 up
		sudo ip netns exec vm2 ip address add $2 dev eth0
	fi
}

function stop() {
	set -x

	sudo ovs-vsctl del-br br-int

	sudo ip l set br-vm1 down
	sudo ip link del vm1-eth0
	sudo ip netns del vm1
	sudo ip l del ovs-vm1-eth0
	sudo brctl delbr br-vm1

	sudo ip link del vm2-eth0
	sudo ip netns del vm2
	sudo ip l set br-vm2 down
	sudo ip l del ovs-vm2-eth0
	sudo brctl delbr br-vm2
}


if [ "$1" == "start" ]; then
	if [ -z "$2" ]; then
		echo "Usage: $0 start <ns ip/mask> [ns2 ip/mask]"
		exit 1
	fi

	start $2 $3
else
	stop
fi
