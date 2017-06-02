#!/bin/bash

DIR=/tmp/netcleanup
CURDIR="$(dirname "$0")"

function cleanup() {
  # cleanup old netns
  if [ -e $DIR/netns.init ] && [ -e $DIR/netns.snapshot ]; then
    grep -v -F -x -f $DIR/{netns.init,netns.snapshot} | while read NETNS; do
      ip netns del $NETNS
    done
    rm -f $DIR/netns
  fi

  # cleanup old interfaces
  if [ -e $DIR/intf.init ] && [ $DIR/intf.snapshot ]; then
    grep -v -F -x -f $DIR/{intf.init,intf.snapshot} | while read INTF; do
      ip link del $INTF
    done
    rm -f $DIR/intf
  fi

  # cleanup old ovsdb
  if [ -e $DIR/ovsdb.init ] && [ $DIR/ovsdb.snapshot ]; then
    grep -v -F -x -f $DIR/{ovsdb.init,ovsdb.snapshot} | while read BRIDGE; do
      ovs-vsctl del-br $BRIDGE
    done
    rm -f $DIR/intf
  fi

  # cleanup old containers
  if [ -e $DIR/docker.init ] && [ $DIR/docker.snapshot ]; then
    grep -v -F -x -f $DIR/{docker.init,docker.snapshot} | while read CONTAINER; do
      docker stop $CONTAINER
      docker rm $CONTAINER
    done
    rm -f $DIR/intf
  fi

  "${CURDIR}/../scale.sh" stop 10 10 10 &> /dev/null

  # clean elasticsearch
  curl -X DELETE http://localhost:9200/skydive

  # clean etcd
  systemctl stop etcd
  rm -rf /var/lib/etcd/default.etcd/

  # clean orientdb
  cat > /tmp/commands.txt <<EOF
DROP DATABASE remote:localhost/Skydive root root plocal
EOF
  /opt/orientdb/bin/console.sh /tmp/commands.txt

  systemctl restart openvswitch
  systemctl restart elasticsearch
  systemctl restart orientdb
  systemctl restart etcd

  rm -rf /tmp/skydive_agent*
}

function init() {
  mkdir -p $DIR

  # save netns
  ip netns | awk '{print $1}' > $DIR/netns.init

  # save interfaces
  ip -o link show | awk -F': ' '{print $2}' | sort > $DIR/intf.init

  # save ovsdb bridges
  ovs-vsctl list-br | sort > $DIR/ovsdb.init

  # save docker containers
  docker ps -a -q | sort > $DIR/docker.init
}

function snapshot() {
  # save netns
  ip netns | awk '{print $1}' > $DIR/netns.snapshot

  # save interfaces
  ip -o link show | awk -F': ' '{print $2}' | sort > $DIR/intf.snapshot

  # save ovsdb bridges
  ovs-vsctl list-br | sort > $DIR/ovsdb.snapshot

  # save docker containers
  docker ps -a -q | sort > $DIR/docker.snapshot
}

case "$1" in
  init)
    init
    ;;

  snapshot)
    snapshot
    ;;

  cleanup)
    cleanup
    ;;

  *)
    echo "Usage: $0 {init|snapshot|cleanup}"
    exit 1
    ;;
esac

exit 0
