#!/bin/bash

function _sudo()
{
    echo -n \$ $*
    read
    sudo $*
}


_sudo ip netns add vm1
_sudo ip netns add vm2
_sudo ip l add vm1-eth0 type veth peer name eth0 netns vm1
_sudo ip l set vm1-eth0 up
_sudo ip l add vm2-eth0 type veth peer name eth0 netns vm2
_sudo ip l set vm2-eth0 up
_sudo ip netns exec vm1 ip l set eth0 up
_sudo ip netns exec vm1 ip a add 192.168.0.1/24 dev eth0
_sudo ip netns exec vm2 ip l set eth0 up
_sudo ip netns exec vm2 ip a add 192.168.0.2/24 dev eth0
_sudo ovs-vsctl add-br br1
_sudo ovs-vsctl add-br br2
_sudo ovs-vsctl add-port br1 vm1-eth0
_sudo ovs-vsctl add-port br2 vm2-eth0
_sudo ip l add vm2-eth1 type veth peer name eth1 netns vm2
_sudo ip l add vm1-eth1 type veth peer name eth1 netns vm1
_sudo brctl addbr lb1
_sudo brctl addif lb1 vm1-eth1
_sudo brctl addif lb1 vm2-eth1
_sudo ovs-vsctl add-port br1 patch-br2 -- set interface patch-br2 type=patch
_sudo ovs-vsctl add-port br2 patch-br1 -- set interface patch-br1 type=patch
_sudo ovs-vsctl set interface patch-br1 option:peer=patch-br2
_sudo ovs-vsctl set interface patch-br2 option:peer=patch-br1
_sudo ovs-vsctl add-br br3
_sudo ovs-vsctl add-port br3 int -- set interface int type=internal
_sudo ip l set int netns vm1

_sudo ovs-vsctl show
_sudo brctl show

_sudo ovs-vsctl del-br br1
_sudo ovs-vsctl del-br br2
_sudo ovs-vsctl del-br br3
_sudo ip netns del vm1
_sudo ip netns del vm2
_sudo ip l del lb1
_sudo ip l del vm1-eth0
_sudo ip l del vm1-eth1
_sudo ip l del vm2-eth0
_sudo ip l del vm2-eth1
