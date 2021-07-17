#!/bin/bash

set -eu

# DEPENDENCIES:
# This scripts assumes the following packages are installed:
#   docker
#   go
#   python and pip
# PERMISSIONS:
# This script can be executed without root privilegeds. 'sudo' will be used
# for the specific tasks that do require it

# Global env vars that modify the behavior of the script:
KIND_VERSION=${KIND_VERSION:-v0.10.0}
K8S_VERSION=${K8S_VERSION:-v1.20.0}
OVNK8S_VERSION=${OVNK8S_VERSION:-ab8bcb41ae215afd3890f243fd99e1d0cce5feed}
OVNK8S_REPO=https://github.com/ovn-org/ovn-kubernetes.git

# Global vars
WORKSPACE=${WORKSPACE:-${HOME}/ovnkube-setup}
OS=linux
ARCH=amd64

usage(){
    echo "Usage: $0 command"
    echo ""
    echo "commands:"
    echo "  create:         Create an environment with kind and ovnk8s"
    echo "  delete:         Delete the environment"
    echo "  start:          Start the environment"
    echo "  stop:           Stop the environment"
}

error() {
    echo "[error] $@"
    exit 1
}

check_dependencies() {
    # Check docker is installed and running
    docker version > /dev/null || (error "Docker not installed"; exit 1)
    # Check go is installed
    go version > /dev/null || (error "Go not installed"; exit 1)
}

do_create () {
    check_dependencies

    # Download needed
    mkdir -p ${WORKSPACE}/bin

    curl -Lo $WORKSPACE/bin/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-$(uname)-amd64"
    chmod +x $WORKSPACE/bin/kind

    curl -Lo ${WORKSPACE}/bin/kubectl "https://storage.googleapis.com/kubernetes-release/release/$K8S_VERSION/bin/$OS/$ARCH/kubectl"
    chmod +x $WORKSPACE/bin/kubectl

    # Need to add port 11337 in the firewall (if it exists)
    if firewall-cmd -q --state; then
        sudo firewall-cmd --add-port=11337/tcp
        sudo firewall-cmd --reload
    fi

    rm -rf $GOPATH/src/github.com/ovn-org
    mkdir -p $GOPATH/src/github.com/ovn-org/
    pushd $GOPATH/src/github.com/ovn-org/
      git clone $OVNK8S_REPO
      pushd ovn-kubernetes
        git checkout $OVNK8S_VERSION
        pushd go-controller
        make
        popd
        pushd dist/images
        make fedora
        popd

        # Disable SCTPSupport
        sed -i '/SCTPSupport.*/d' contrib/kind.yaml.j2
        sed -i '/featureGates.*/d' contrib/kind.yaml.j2

      popd # ovn-kubernetes
    popd #$GOPATH/src/github.com/ovn-org/
}

do_delete() {
    do_stop
    rm -rf ${WORKSPACE}/go/src/github.com/ovn-org/ovn-kubernetes
    docker rmi ovn-kube-f || true
    docker network rm kind || true
    rm -f $HOME/admin.conf
}

do_start() {
    pushd "$GOPATH/src/github.com/ovn-org/ovn-kubernetes/contrib"
    (
        K8S_VERSION=$K8S_VERSION KIND_INSTALL_INGRESS=true PATH="$PATH:${WORKSPACE}/bin" ./kind.sh
    )
    popd
}

do_stop() {
    if [ -d "$GOPATH/src/github.com/ovn-org/ovn-kubernetes/contrib" ]; then
        pushd "$GOPATH/src/github.com/ovn-org/ovn-kubernetes/contrib"
        (
            K8S_VERSION=$K8S_VERSION KIND_INSTALL_INGRESS=true PATH="$PATH:${WORKSPACE}/bin" ./kind.sh --delete
        )
    fi
}

# Main program
if [ $# -ne 1 ]; then
    usage
    exit 1
fi
CMD=$1
case $CMD in
    start)
        do_start $@
        ;;
    stop)
        do_stop $@
        ;;
    create)
        do_create $@
        ;;
    delete)
        do_delete $@
        ;;
    forward)
        do_forward $@ # NEEDED?
        ;;
    *)
        echo "Invalid command $CMD" 1>&2
        exit 1
        ;;
esac
