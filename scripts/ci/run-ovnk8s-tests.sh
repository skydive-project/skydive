#!/bin/bash

set -v
set -e

SCRIPT_FILE=$(realpath -s $0)
DIR="$(dirname $SCRIPT_FILE)"

. "$DIR/run-tests-utils.sh"

OVN_ADDRESS=""

ovnk8s_setup_start() {
    $DIR/ovnkube-setup.sh create
    $DIR/ovnkube-setup.sh start
}

ovnk8s_setup_stop() {
    $DIR/ovnkube-setup.sh stop
    $DIR/ovnkube-setup.sh delete
}

ovnk8s_setup_start
OVN_NODE_IP=$(KUBECONFIG=$HOME/admin.conf kubectl get nodes  -o wide | awk '/ovn-control-plane/{print $6}')
export OVN_ADDRESS="tcp:${OVN_NODE_IP}:6641"
export K8S_CLUSTER_NAME="kind-ovn"
export K8S_NUM_NODES=3
export KUBECONFIG=$HOME/admin.conf

network_setup
WITH_OVNK8S=true
TEST_PATTERN='(OVNK8s|K8s)'
tests_run

ovnk8s_setup_stop

exit $RETCODE

