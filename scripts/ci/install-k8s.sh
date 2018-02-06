#!/bin/bash

set -v

OS=linux
ARCH=amd64
TARGET_DIR="/usr/bin"

MINIKUBE_VERSION="v0.24.1"
MINIKUBE_URL="https://github.com/kubernetes/minikube/releases/download/$MINIKUBE_VERSION/minikube-$OS-$ARCH"

KUBECTL_VERSION="v1.9.0"
KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/$OS/$ARCH/kubectl"

uninstall_binary() {
	local prog=$1
	sudo rm -f ${TARGET_DIR}/$prog
}

install_binary() {
	local prog=$1
	local url=$2
	wget --no-check-certificate -O $prog $url
	chmod a+x $prog
	sudo mv -f $prog ${TARGET_DIR}/.
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
	sudo minikube delete || true
	sudo rm -rf ~/.minikube
}

start() {
	sudo CHANGE_MINIKUBE_NONE_USER=true minikube --vm-driver=none start
	sudo minikube status
	kubectl config use-context minikube
}

status() {
	kubectl version
	kubectl config get-contexts
}

case "$1" in
	start)
		stop
		uninstall
		install
		start
		;;
	stop)
		stop
		uninstall
		;;
	status)
		status
		;;
	*)
		echo "$0 [start|stop|status]"
		exit 1
		;;
esac

exit 0
