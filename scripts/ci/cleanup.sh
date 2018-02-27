#!/bin/bash

DIR=/tmp/netcleanup
CURDIR="$(dirname "$0")"

function cleanup_items() {
  local label=$1
  local cmd="$2"
  if [ -e $DIR/$label.init ] && [ -e $DIR/$label.snapshot ]; then
    grep -v -F -x -f $DIR/{$label.init,$label.snapshot} | while read i; do
      eval "$cmd $i"
    done
    rm -f $DIR/$label.snapshot
  fi
}

function docker_rm() {
  local container=$1
  docker stop $container
  for i in $( seq 5 ); do 
    docker rm -f $container && break || sleep 1
  done
}

function cleanup() {
  # cleanup minikube
  "${CURDIR}/install-minikube.sh" stop

  cleanup_items netns "ip netns del"
  cleanup_items intf "ip link del"
  cleanup_items ovsdb "ovs-vsctl del-br"
  cleanup_items docker "docker_rm"
  cleanup_items lxd "lxc delete --force"
  cleanup_items docker-images "docker rmi -f"

  "${CURDIR}/../scale.sh" stop 10 10 10

  # clean elasticsearch
  curl -X DELETE 'http://localhost:9200/skydive*'

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

  virsh net-destroy vagrant0
  virsh net-destroy vagrant-libvirt

  rm -rf /tmp/skydive_agent* /tmp/skydive-etcd

  # time to restart services
  sleep 8
}

function snapshot_items() {
  local label=$1
  local ext=$2
  local cmd="$3"
  eval "$cmd | sort | tee $DIR/$label.$ext"
}

function snapshot() {
  local ext=$1
  mkdir -p $DIR

  snapshot_items netns $ext "ip netns | awk '{print \$1}'"
  snapshot_items intf $ext "ip -o link show | awk -F': ' '{print \$2}' | cut -d '@' -f 1"
  snapshot_items ovsdb $ext "ovs-vsctl list-br"
  snapshot_items docker $ext "docker ps -a -q"
  snapshot_items docker-images $ext "docker images -a -q"
  snapshot_items lxd $ext "lxc list --format csv -c n"
}

case "$1" in
  init)
    snapshot init
    ;;

  snapshot)
    snapshot snapshot
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
