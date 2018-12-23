#!/bin/bash

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
BOOKINFO_URL=https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo

if [ -z "$NAMESPACE" ]; then
	NAMESPACE=default
fi
echo "export NAMESPACE=$NAMESPACE"

if [ -z "$WITH_ISTIO" ]; then
	WITH_ISTIO=false
fi
echo "export WITH_ISTIO=$WITH_ISTIO"

if [ -z "$WITH_NETPOL_FIX" ]; then
	WITH_NETPOL_FIX=true
fi
echo "export WITH_NETPOL_FIX=$WITH_NETPOL_FIX"

if [ -z "$SRVNAME" ]; then
	SRVNAME=productpage
fi
echo "export SRVNAME=$SRVNAME"

apply() {
	kubectl apply -n $NAMESPACE -f $1
}

delete() {
	kubectl delete --grace-period=0 --force -n $NAMESPACE -f $1
}

stop() {
	$WITH_ISTIO && delete $BOOKINFO_URL/networking/destination-rule-all.yaml
	$WITH_ISTIO && delete $BOOKINFO_URL/networking/bookinfo-gateway.yaml
	delete $BOOKINFO_URL/platform/kube/bookinfo.yaml
	delete $BOOKINFO_URL/platform/kube/bookinfo-ingress.yaml
	delete $SCRIPTDIR/bookinfo-networkpolicy.yaml
	$WITH_NETPOL_FIX && delete $SCRIPTDIR/bookinfo-networkpolicy-fix.yaml

	kubectl delete namespace $NAMESPACE
}

start() {
	kubectl create namespace $NAMESPACE

	$WITH_ISTIO && apply $BOOKINFO_URL/networking/destination-rule-all.yaml
	$WITH_ISTIO && apply $BOOKINFO_URL/networking/bookinfo-gateway.yaml
	apply $BOOKINFO_URL/platform/kube/bookinfo.yaml
	apply $BOOKINFO_URL/platform/kube/bookinfo-ingress.yaml
	apply $SCRIPTDIR/bookinfo-networkpolicy.yaml
	$WITH_NETPOL_FIX && apply $SCRIPTDIR/bookinfo-networkpolicy-fix.yaml
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
