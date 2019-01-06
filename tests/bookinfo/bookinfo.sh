#!/bin/bash

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
BOOKINFO_URL=https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo

[ -z "$NAMESPACE" ] && NAMESPACE=default
echo "export NAMESPACE=$NAMESPACE"

[ -z "$WITH_ISTIO" ] && WITH_ISTIO=false
echo "export WITH_ISTIO=$WITH_ISTIO"

[ -z "$WITH_NETPOL" ] && WITH_NETPOL=false
echo "export WITH_NETPOL=$WITH_NETPOL"

[ -z "$WITH_IBMCLOUD" ] && WITH_IBMCLOUD=false
echo "export WITH_IBMCLOUD=$WITH_IBMCLOUD"

[ -z "$SRVNAME" ] && SRVNAME=productpage
echo "export SRVNAME=$SRVNAME"

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

bookinfo() {
	$WITH_IBMCLOUD && $1 $SCRIPTDIR/*ibmcloud*.yaml
	$WITH_ISTIO && $1 $BOOKINFO_URL/networking/destination-rule-all.yaml
	$WITH_ISTIO && $1 $BOOKINFO_URL/networking/bookinfo-gateway.yaml
	$1 $BOOKINFO_URL/platform/kube/bookinfo.yaml
	$1 $BOOKINFO_URL/platform/kube/bookinfo-ingress.yaml
	$WITH_NETPOL && $1 $SCRIPTDIR/*netpol*.yaml
}

stop() {
	bookinfo delete
	kubectl delete namespace $NAMESPACE
}

start() {
	kubectl create namespace $NAMESPACE
	bookinfo create
}

port_forward() {
	srvport=$(kubectl get service/$SRVNAME -n $NAMESPACE -o jsonpath='{.spec.ports[0].port}')
	kubectl port-forward service/$SRVNAME -n $NAMESPACE $srvport:$srvport
}

case "$1" in
	stop)
		stop
		;;
	start)
		start
		;;
	port-forward)
		port_forward
		;;
	*)
		echo "$0 [stop|start|port-forward|help]"
		exit 1
		;;
esac
exit 0
