#!/bin/bash

# create/delete a test topology
# syntax:
#   ./tunnel.sh start gre|vxlan|geneve
#   ./tunnel.sh stop gre|vxlan|geneve

function start() {
	set -x

	sudo ovs-vsctl add-br br-${1}

	sudo ip netns add vm1
	sudo ip link add vm1-eth0 type veth peer name eth0 netns vm1
	sudo ip link set vm1-eth0 up

	sudo ip netns exec vm1 ip link set eth0 up
	sudo ip netns exec vm1 ip address add 172.16.0.1/24 dev eth0

	sudo ip netns add vm2
	sudo ip link add vm2-eth0 type veth peer name eth0 netns vm2
	sudo ip link set vm2-eth0 up

	sudo ip netns exec vm2 ip link set eth0 up
	sudo ip netns exec vm2 ip address add 172.16.0.2/24 dev eth0

	sudo ovs-vsctl add-port br-${1} vm1-eth0
	sudo ovs-vsctl add-port br-${1} vm2-eth0

	
	if [ "$1" == "gre" ]; then
	    sudo ip netns exec vm1 ip tunnel add ${1} mode gre remote 172.16.0.2 local 172.16.0.1 ttl 255
	elif [ "$1" == "geneve" ]; then
	    sudo ip netns exec vm1 ip link add ${1} type geneve id 10 remote 172.16.0.2
	else
	    sudo ip netns exec vm1 ip link add ${1} type vxlan id 10 group 239.0.0.10 ttl 10 dev eth0 dstport 4789
	fi
	sudo ip netns exec vm1 ip l set ${1} up
	sudo ip netns exec vm1 ip link add name in type dummy
	sudo ip netns exec vm1 ip l set in up
	sudo ip netns exec vm1 ip a add 192.168.0.1/32 dev in
	sudo ip netns exec vm1 ip r add 192.168.0.0/24 dev ${1}

	if [ "$1" == "gre" ]; then
	    sudo ip netns exec vm2 ip tunnel add ${1} mode gre remote 172.16.0.1 local 172.16.0.2 ttl 255
	elif [ "$1" == "geneve" ]; then
	    sudo ip netns exec vm2 ip link add ${1} type geneve id 10 remote 172.16.0.1
	else
	    sudo ip netns exec vm2 ip link add ${1} type vxlan id 10 group 239.0.0.10 ttl 10 dev eth0 dstport 4789
	fi
	sudo ip netns exec vm2 ip l set ${1} up
	sudo ip netns exec vm2 ip link add name in type dummy
	sudo ip netns exec vm2 ip l set in up
	sudo ip netns exec vm2 ip a add 192.168.0.2/32 dev in
	sudo ip netns exec vm2 ip r add 192.168.0.0/24 dev ${1}
}

function stop() {
	set -x

	sudo ovs-vsctl del-br br-${1}

	sudo ip link del vm1-eth0
	sudo ip netns del vm1
	sudo ip link del vm2-eth0
	sudo ip netns del vm2
}

if [ "$2" != "gre" ] && [ "$2" != "vxlan" ] && [ "$2" != "geneve" ]
then
    echo -n "Second argument must be 'gre' or 'vxlan'. Exiting."
    exit 1
fi

if [ "$1" == "start" ]; then
	start $2
else
	stop $2
fi
