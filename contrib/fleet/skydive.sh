#/usr/bin/env bash

set -x
set -e

WITHOUT_BROKER=${WITHOUT_BROKER:-false}
WITHOUT_SPOKE=${WITHOUT_SPOKE:-false}
K3D_AGENTS=${K3D_AGENTS:-0}
FLEET_VERSION=${FLEET_VERSION:-0.3.0}
ANALYZER_NODEPORTS=${ANALYZER_NODEPORTS:-30082}
ETCD_NODEPORTS=${ETCD_NODEPORTS:-30079}

GIT_REPO=https://github.com/hunchback/skydive/
GIT_BRANCH=charts-fleet
GIT_PATH=contrib/charts/skydive-analyzer
GIT_APP=skydive-analyzer

K3D_SPOKES=${K3D_SPOKES:-east west}

k3d_install() {
	which k3d 2>/dev/null && return
	curl -s https://raw.githubusercontent.com/rancher/k3d/main/install.sh | TAG=v3.0.0 bash
}

tools() {
	k3d_install
}

k3d_delete() {
	local name=$1
	k3d cluster delete $name 2>/dev/null || true
}

k3d_coredns_broker_entry() {
	local hostname=broker
	local ipaddr=$(hostname -I)
	local nodehosts="$(kubectl get configmap coredns -n kube-system -o json | jq -r .data.NodeHosts)"
	kubectl patch configmap coredns -n kube-system -p \
"{
  \"data\":{
    \"NodeHosts\":
      \"${nodehosts}\n $ipaddr $hostname\n\"
  }
}"
	kubectl get configmap coredns -n kube-system -o json | jq .data.NodeHosts
}

k3d_create() {
	local name=$1; shift
	k3d cluster create --agents $K3D_AGENTS $name $*
	sleep 5
	k3d_coredns_broker_entry
}

helm_uninstall() {
	local context=$1; shift
	local name=$1; shift
	helm --kube-context $context --namespace fleet-system uninstall $name 2>/dev/null || true
}

helm_install() {
	local context=$1; shift
	local name=$1; shift
	helm --kube-context $context --namespace fleet-system install --create-namespace $* \
	    $name https://github.com/rancher/fleet/releases/download/v${FLEET_VERSION}/${name}-${FLEET_VERSION}.tgz 
}

helm_status() {
	local context=$1; shift
	helm --kube-context $context --namespace fleet-system list
}

####################
#####  broker ######
####################

broker_delete() {
	k3d_delete broker
}

broker_create() {
	k3d_create broker \
		-p "${ANALYZER_NODEPORTS}:${ANALYZER_NODEPORTS}@server[0]" \
		-p "${ETCD_NODEPORTS}:${ETCD_NODEPORTS}@server[0]"
}

broker_install() {
	local context=k3d-broker
	local url=broker:8080
	helm_install $context fleet-crd
	helm_install $context fleet \
		--wait \
		--set apiServerURL=$url
}

broker_status() {
	local context=k3d-broker
	helm_status $context
}

broker_token() {
	local context=k3d-broker
	local name=token
	local namespace=clusters

	cat <<EOF | kubectl apply --context $context --wait -f - 
apiVersion: v1
kind: Namespace
metadata:
  name: $namespace
---
kind: ClusterRegistrationToken
apiVersion: "fleet.cattle.io/v1alpha1"
metadata:
    name: $name
    namespace: $namespace
spec:
    ttl: 240h
EOF
	kubectl --context $context --namespace $namespace get secret $name -o 'jsonpath={.data.values}' | \
		base64 --decode > /tmp/values.yaml
	cat /tmp/values.yaml
}

broker_proxy_kill() {
	sudo killall -9 kubectl 2>/dev/null || true
}

broker_proxy() {
	local context=k3d-broker
	kubectl proxy --accept-hosts '^.*' --address 0.0.0.0 --port 8080 --context $context&
}

broker_logs() {
	local context=k3d-broker
	kubectl --context $context --namespace fleet-system logs -l app=fleet-controller || true
	kubectl --context $context --namespace fleet-system get pods -l app=fleet-controller || true
}

broker() {
	broker_create
	broker_install
	broker_status
	broker_token
	broker_proxy
}

####################
#####  spoke  ######
####################

spoke_delete() {
	local name=$1; shift
	k3d_delete $name
}

spoke_create() {
	local name=$1; shift
	k3d_create $name
}

spoke_install() {
	local name=$1; shift
	local context=k3d-$name
	helm_install $context fleet-crd
	helm_install $context fleet-agent \
		--set-string labels.env=$name \
		--values /tmp/values.yaml
}

spoke_status() {
	local name=$1; shift
	local context=k3d-$name
	helm_status $context
}

spoke_logs() {
	local name=$1; shift
	local context=k3d-$name
	kubectl --context $context --namespace fleet-system logs -l app=fleet-agent || true
	kubectl --context $context --namespace fleet-system get pods -l app=fleet-agent || true
}

spoke() {
	local name=$1; shift
	spoke_delete $name
	spoke_create $name
	spoke_install $name
}

####################
#####   app   ######
####################

gitrepo_create() {
	local context=k3d-broker
	local namespace=$1; shift
	kubectl --context $context create namespace $namespace 2>/dev/null || true
	cat <<EOF | kubectl apply --context $context -f -
kind: GitRepo
apiVersion: fleet.cattle.io/v1alpha1
metadata:
  name: sample
  namespace: $namespace
spec:
  repo: $GIT_REPO
  branch: $GIT_BRANCH
  paths:
  - $GIT_PATH
  targets:
  - name: local
    clusterSelector:
      matchLabels:
        name: local
  - name: east
    clusterSelector:
      matchLabels:
        env: east
  - name: west
    clusterSelector:
      matchLabels:
        env: west
EOF
}

gitrepo_status() {
	local context=k3d-broker
	local namespace=$1; shift
	kubectl --context $context --namespace $namespace get fleet
}

broker_app_install() {
	gitrepo_create fleet-local
}

spoke_app_install() {
	gitrepo_create clusters
}

app_status() {
	local context=k3d-$1; shift
	local namespace=default
	local name=$GIT_APP
	#kubectl --context $context rollout status deploy --namespace $namespace $name --timeout 30s || true
	kubectl --context $context get deploy --namespace $namespace $name || true
}

####################
#####  main   ######
####################

uninstall() {
	broker_proxy_kill
	broker_delete
	for i in $K3D_SPOKES; do
		spoke_delete $i
	done
}

install() {
	broker
	$WITHOUT_BROKER || broker_app_install
	for i in $K3D_SPOKES; do
		spoke $i
	done
	$WITHOUT_SPOKE || spoke_app_install
}

state() {
	broker_status
	for i in $K3D_SPOKES; do
		spoke_status $i
	done

	$WITHOUT_SPOKE || gitrepo_status fleet-local
	$WITHOUT_SPOKE || gitrepo_status clusters

	$WITHOUT_BROKER || app_status broker
	for i in $K3D_SPOKES; do
		$WITHOUT_SPOKE || app_status $i
	done
}

logs() {
	broker_logs
	for i in $K3D_SPOKES; do
		$WITHOUT_SPOKE || spoke_logs $i
	done
}

case "$1" in 
        tools)
		tools
		;;
	install)
		install
		;;
	uninstall)
		uninstall
		;;
	status)
		state
		;;
	logs)
		logs
		;;
	*)
		echo "usage: skydive.sh [tools|install|uninstall|status|logs]"
esac
