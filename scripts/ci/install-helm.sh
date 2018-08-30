#!/bin/bash

export HELM_INSTALL_DIR=/usr/bin
HELM_VER="v2.9.1"
HELM_GET=https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get
helm='helm --tiller-connection-timeout 10'

uninstall() {
	sudo rm -rf $HELM_INSTALL_DIR/helm
}

install() {
	curl $HELM_GET | sed 's/helm version/helm --debug version/' | sh -s -- --version $HELM_VER
}

stop() {
	$helm reset --force || true
}

start() {
	$helm reset --force
	$helm init --service-account default
}

status() {
	kubectl version
	kubectl get nodes
	kubectl get pods --all-namespaces
	$helm version
	$helm list
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
