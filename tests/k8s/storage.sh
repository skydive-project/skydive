#!/bin/bash
#
# this is driver to running the scenario found here:
# https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage/

SRC=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

: ${NAMESPACE:=default}
echo "export NAMESPACE=$NAMESPACE"

DATA=/mnt/skydive-test-storage

setup() {
	sudo mkdir -p $DATA
	sudo chmod 777 $DATA
	echo 'Hello from kubernetes storage' > $DATA/index.html
}

cleanup() {
	sudo rm -rf $DATA
}

create() {
	for file in "$@"; do
		kubectl apply -n $NAMESPACE -f $file
		shift
	done
}

delete() {
	for file in "$@"; do
		kubectl delete --grace-period=0 --force -n $NAMESPACE -f $file
		shift
	done
}

stop() {
	delete $SRC/pv-pod.yaml
	delete $SRC/pv-claim.yaml
	delete $SRC/pv-volume.yaml
	kubectl delete namespace $NAMESPACE
	cleanup
}

start() {
	setup
	kubectl create namespace $NAMESPACE
	create $SRC/pv-volume.yaml
	create $SRC/pv-claim.yaml
	create $SRC/pv-pod.yaml
}

case "$1" in
	stop)
		stop
		;;
	start)
		start
		;;
	*)
		echo "$0 [stop|start|help]"
		exit 1
		;;
esac
exit 0
