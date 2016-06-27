#!/bin/bash

# create/delete a test topology
# syntax:
#   ./simple.sh start <ns ip/mask> [ns2 ip/mask]
#   ./simple.sh stop
#
# options:
#   ns2 ip/mask: create a second ns

function start_ns() {
    NAME=ns_$1
    IP=192.168.0.$1/24

    sudo ip netns add $NAME
    sudo ip link add ${NAME}-eth0 type veth peer name eth0 netns $NAME
    sudo ip link set ${NAME}-eth0 up

    sudo ip netns exec $NAME ip link set eth0 up
    sudo ip netns exec $NAME ip address add $IP dev eth0

#    sudo brctl addif br001 ${NAME}-eth0
    sudo ovs-vsctl add-port br001 ${NAME}-eth0
}

function stop_ns() {
    NAME=ns_$1

	sudo ip link del ${NAME}-eth0
	sudo ip netns del ${NAME}
}


function start() {
	set -x
    NUMBER=$1

#	sudo brctl addbr br001
    sudo ovs-vsctl add-br br001

    for i in `seq $NUMBER`; do
        start_ns $i
    done
}

function stop() {
	set -x
    NUMBER=$1

#	sudo brctl delbr br001
    sudo ovs-vsctl del-br br001

    for i in `seq $NUMBER`; do
        stop_ns $i
    done
}

if [ -z "$2" ]; then
	echo "Usage: $0 <start/stop> <number>"
	exit 1
fi

if [ "$1" == "start" ]; then
	start $2
else
	stop $2
fi
