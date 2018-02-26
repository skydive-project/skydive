#!/bin/bash

set -v

OS=linux
ARCH=amd64
TARGET_DIR="/usr/bin"

MINIKUBE_VERSION="v0.24.1"
MINIKUBE_URL="https://github.com/kubernetes/minikube/releases/download/$MINIKUBE_VERSION/minikube-$OS-$ARCH"

KUBECTL_VERSION="v1.9.0"
KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/$OS/$ARCH/kubectl"

[ -z "$MINIKUBE_FLAGS" ] && MINIKUBE_FLAGS="--extra-config=kubelet.CgroupDriver=systemd"
[ -z "$MINIKUBE_VM_DRIVER" ] && MINIKUBE_VM_DRIVER=none

uninstall_binary() {
	local prog=$1
	sudo rm -f ${TARGET_DIR}/$prog
}

install_binary() {
	local prog=$1
	local url=$2
	local target=${TARGET_DIR}/$prog

	[ -f $target ] && return

	wget --no-check-certificate -O $prog $url
	if [ $? != 0 ]; then
		echo "failed to download $url"
		exit 1
	fi

	chmod a+x $prog
	sudo mv -f $prog $target
}

check_minikube() {
	which minikube 2>/dev/null
	if [ $? != 0 ]; then
		echo "minikube is not installed. Please run install-minikube.sh install"
		exit 1
	fi
}

install() {
	install_binary minikube $MINIKUBE_URL
	install_binary kubectl $KUBECTL_URL
}

uninstall() {
	uninstall_binary minikube
	uninstall_binary kubectl
}

stop() {
	check_minikube
	sudo $(which minikube) delete || true
	sudo rm -rf ~/.minikube ~/.kube
}

start() {
	check_minikube
	sudo -E CHANGE_MINIKUBE_NONE_USER=true $(which minikube) \
		--vm-driver=${MINIKUBE_VM_DRIVER} \
		${MINIKUBE_FLAGS} start
	sudo $(which minikube) status
	kubectl config use-context minikube
}

status() {
	kubectl version
	kubectl config get-contexts
}

case "$1" in
	install)
		install
		;;
	uninstall)
		uninstall
		;;
	start)
		start
		;;
	stop)
		stop
		;;
	status)
		status
		;;
	*)
		echo "$0 [install|uninstall|start|stop|status]"
		exit 1
		;;
esac
