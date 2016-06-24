#!/bin/bash

# create/delete a test topology
# syntax:
#   ./scale.sh start <number>
#   ./scale.sh stop <number>
#

function ns_start() {
	set -x

  NUM=$1
  NAME=vm$NUM

	sudo ip netns add ${NAME}
	sudo ip link add ${NAME}-eth0 type veth peer name eth0 netns ${NAME}
	sudo ip link set ${NAME}-eth0 up

	sudo ip netns exec ${NAME} ip link set eth0 up
	sudo ip netns exec ${NAME} ip address add 192.168.0.${NUM} dev eth0

	sudo ovs-vsctl add-port br-int ${NAME}-eth0
}

function ns_stop() {
	set -x

  NUM=$1
  NAME=vm$NUM

	sudo ip link del ${NAME}-eth0
	sudo ip netns del ${NAME}
}

function start() {
	NUM=$1

	sudo ovs-vsctl add-br br-int

 	for i in $( seq $NUM ); do
		ns_start $i
	done
}

function stop() {
	NUM=$1

	sudo ovs-vsctl del-br br-int

	for i in $( seq $NUM ); do
		ns_stop $i
	done
}

if [ -z "$2" ]; then
	echo "Usage: $0 start/stop <number>"
	exit 1
fi

if [ "$1" == "start" ]; then
	start $2
else
	stop $2
fi
