#!/bin/bash

set -v

OS=linux
ARCH=amd64
TARGET_DIR="/usr/local/bin"

MINIKUBE_VERSION="v0.24.1"
MINIKUBE_URL="https://github.com/kubernetes/minikube/releases/download/$MINIKUBE_VERSION/minikube-$OS-$ARCH"

KUBECTL_VERSION="v1.9.0"
KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/$OS/$ARCH/kubectl"

[ -z "$MINIKUBE_FLAGS" ] && MINIKUBE_FLAGS=--extra-config=kubelet.CgroupDriver=systemd
[ -z "$MINIKUBE_VM_DRIVER" ] && MINIKUBE_VM_DRIVER=none

do_sudo() {
	sudo $@
}

do_uninstall() {
	local prog=$1
	sudo rm -f ${TARGET_DIR}/$prog
}

do_install() {
	local prog=$1
	local url=$2
	wget --no-check-certificate -O $prog $url
	chmod a+x $prog
	do_sudo mv -f $prog ${TARGET_DIR}/.
}

do_minikube() {
	if [ ! -f ${TARGET_DIR}/minikube ]
	then
		echo "minikube is not installed. Please run install-minikube.sh install"
		exit 1
	fi
	do_sudo ${TARGET_DIR}/minikube $@
}

install() {
	[ ! -f ${TARGET_DIR}/minikube ] && do_install minikube $MINIKUBE_URL
	[ ! -f ${TARGET_DIR}/kubectl ] && do_install kubectl $KUBECTL_URL
}

uninstall() {
	do_uninstall minikube
	do_uninstall kubectl
}

stop() {
	do_minikube delete || true
	do_sudo rm -rf ~/.minikube ~/.kube
}

start() {
	CHANGE_MINIKUBE_NONE_USER=true do_minikube --vm-driver=${MINIKUBE_VM_DRIVER} ${MINIKUBE_FLAGS} start
	do_minikube status
	do_sudo rm -rf $HOME/.kube $HOME/.minikube
	do_sudo cp -r /root/.kube $HOME/.kube
	do_sudo chown -R $USER $HOME/.kube
	do_sudo chgrp -R $USER $HOME/.kube
	do_sudo cp -r /root/.minikube $HOME/.minikube
	do_sudo chown -R $USER $HOME/.minikube
	do_sudo chgrp -R $USER $HOME/.minikube
	sed -i s:/root:$HOME: $HOME/.kube/config
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
