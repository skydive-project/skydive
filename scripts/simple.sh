#!/bin/bash

# create/delete a test topology
# syntax:
#   ./simple.sh start [-d|--delay] [-b|--bridge] s[<ns ip/mask> [ns2 ip/mask]]"
#   ./simple.sh stop  [-d|--delay] [-b|--bridge]"
#   ./simple.sh ping  <ns ip> <ns2 ip>"

IP1=10.0.0.1
IP2=10.0.0.2

SUBNET1=$IP1/24
SUBNET2=$IP2/24

DELAY1=100ms
DELAY2=200ms
DELAY3=400ms
DELAY4=800ms

NS1=vm1
NS2=vm2

function vm_start() {
	local vm=$1
	local ip=$2
	local delayhost=$3
	local delayvm=$4

	local if=eth0
	local vmif=$vm-$if
	local ovsif=ovs-$vm-$if
	local br=br-$vm
	local brif=$br-$if

	sudo ip netns add $vm

	if [ -n "$BRIDGE" ]; then
		sudo brctl addbr $br
		sudo ip link set $br up

		sudo ip link add $ovsif type veth peer name $brif
		sudo ip link set $ovsif up
		sudo ovs-vsctl add-port br-int $ovsif

		sudo brctl addif $br $brif
		sudo ip link set $brif up
	fi

	sudo ip link add $vmif type veth peer name $if netns $vm

	if [ -n "$BRIDGE" ]; then
		sudo brctl addif $br $vmif
	fi

	sudo ip link set $vmif up

	if [ -z "$BRIDGE" ]; then
		sudo ovs-vsctl add-port br-int $vmif
	fi

	if [ -n "$DELAY" ]; then
	      	sudo tc qdisc add dev $vmif root netem delay $delayhost
	fi

	sudo ip netns exec $vm ip link set $if up
	sudo ip netns exec $vm ip address add $ip dev $if

	if [ -n "$DELAY" ]; then
		sudo ip netns exec $vm tc qdisc add dev $if root netem delay $delayvm
	fi
}

function vm_stop() {
	local vm=$1

	local if=eth0
	local vmif=$vm-$if
	local ovsif=ovs-$vm-$if
	local br=br-$vm
	local brif=$br-$if

	if [ -n "$DELAY" ]; then
		sudo tc qdisc del dev $vmif root
	fi

	sudo ip link set $vmif down
	sudo ip link del $vmif

	if [ -n "$BRIDGE" ]; then
		sudo ip link set $brif down 
		sudo brctl delif $br $brif

		sudo ip link set $ovsif down
		sudo ip link del $ovsif

		sudo ip link set $br down
		sudo brctl delbr $br
	fi

	sudo ip netns del $vm
}

function start() {
	set -x
	sudo ovs-vsctl add-br br-int
	vm_start $NS1 $SUBNET1 $DELAY2 $DELAY1
	[ -n "$SUBNET2" ] && vm_start $NS2 $SUBNET2 $DELAY3 $DELAY4
}

function stop() {
	set -x
	sudo ovs-vsctl del-br br-int
	vm_stop $NS1
	vm_stop $NS2
}

function ping() {
	echo
	echo "NOTE: Expect RTT is ($DELAY1 + $DELAY2 + $DELAY3 + $DELAY4)."
	echo "NOTE: During 1st run RTT is doubled due to ARP resolution."
	echo
	set -x
	[ -n "$IP2" ] && sudo ip netns exec $NS1 ping -c 1 $IP2
	sudo ip netns exec $NS2 ping -c 1 $IP1
}

function usage() {
	echo "Usage: $0 start [-d|--delay] [-b|--bridge] s[<ns ip/mask> [ns2 ip/mask]]"
	echo "Usage: $0 stop  [-d|--delay] [-b|--bridge]"
	echo "Usage: $0 ping  <ns ip> <ns2 ip>"
	exit 1
}

POSITIONAL=()
while [[ $# -gt 0 ]]; do
	key="$1"
	case $key in
		-d|--delay)
			DELAY=YES
			shift
			;;
		-b|--bridge)
			BRIDGE=YES
			shift
			;;
		*)    # unknown option
			POSITIONAL+=("$1")
			shift
			;;
	esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

case $1 in
	start)
		if [ -n "$2" ]; then
			SUBNET1=$2
			SUBNET2=$3
		fi
		start
		;;
	stop)
		stop
		;;
	ping)
		if [ -n "$2" ]; then
			IP1=$2
			IP2=$3
		fi
		ping
		;;
	*)
		usage
		;;
esac
