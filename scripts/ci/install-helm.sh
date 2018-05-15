#!/bin/bash

export HELM_INSTALL_DIR=/usr/bin
HELM_GET=https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get

uninstall() {
	sudo rm -rf $HELM_INSTALL_DIR/helm
}

install() {
	sudo yum install -y socat
	local runme=/tmp/get_helm.sh
	curl $HELM_GET | sed 's/helm version/helm --debug version/' | sh
}

stop() {
	helm reset
}

start() {
	helm init --upgrade
}

status() {
	kubectl version
	kubectl get nodes
	kubectl get pods --all-namespaces
	helm version --debug
	helm list
}

case "$*" in
	uninstall)
		uninstall
		;;
	install)
		install
		;;
	stop)
		stop
		;;
	start)
		start
		;;
	status)
		status
		;;
	*)
		echo "usage: $0 [uninstall|install|stop|start|status]"
		;;
esac
