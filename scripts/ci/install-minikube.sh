#!/bin/bash

DIR="$(dirname "$0")"

OS=linux
ARCH=amd64
TARGET_DIR=/usr/bin

MINIKUBE_VERSION="v1.1.1"
MINIKUBE_URL="https://github.com/kubernetes/minikube/releases/download/$MINIKUBE_VERSION/minikube-$OS-$ARCH"

K8S_VERSION="v1.14.3"
KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/$K8S_VERSION/bin/$OS/$ARCH/kubectl"

[ -z "$WITH_CALICO" ] && WITH_CALICO=false

CALICO_VERSION="v2.6"
CALICO_URL="https://docs.projectcalico.org/$CALICO_VERSION/getting-started/kubernetes/installation/hosted/calico.yaml"

export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_WANTREPORTERRORPROMPT=false

case "$MINIKUBE_DRIVER" in
        "" | "none")
                MINIKUBE_DRIVER=none
                export MINIKUBE_HOME=$HOME
                export CHANGE_MINIKUBE_NONE_USER=true
                export KUBECONFIG=$HOME/.kube/config
                minikube() { sudo -E minikube $@; }
                kubectl() { sudo -E kubectl $@; }
                ;;
        "virtualbox")
                ;;
        *)
                echo "don't support MINIKUBE_DRIVER value '$MINIKUBE_DRIVER'"
                exit 1
                ;;
esac

uninstall_binary() {
        local prog=$1
        sudo rm -f $TARGET_DIR/$prog
}

install_binary() {
        local prog=$1
        local url=$2

        wget --no-check-certificate -O $prog $url
        if [ $? != 0 ]; then
                echo "failed to download $url"
                exit 1
        fi

        chmod a+x $prog
        sudo mv $prog $TARGET_DIR/$prog
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

        minikube delete || true
        sudo rm -rf $HOME/.minikube $HOME/.kube
        sudo rm -rf /root/.minikube /root/.kube

        if [ "$MINIKUBE_DRIVER" == "none" ]; then
                sudo rm -rf /etc/kubernetes
                sudo rm -rf /var/lib/localkube

                sudo docker system prune -af || true
                for i in $(sudo docker ps -aq --filter name=k8s || true); do
                        sudo docker stop $i || true
                        sudo docker rm $i || true
                done

                sudo systemctl stop localkube || true
                sudo systemctl disable localkube || true
        fi
}

start() {
        check_minikube

        local args="--kubernetes-version $K8S_VERSION --memory 4096"
        if [ "$MINIKUBE_DRIVER" == "none" ]; then
                args="$args --vm-driver=none"
                local driver=$(sudo docker info --format '{{print .CgroupDriver}}')
                if [ -n "$driver" ]; then
                        args="$args --extra-config=kubelet.cgroup-driver=$driver"
                fi
        fi

	$WITH_CALICO && args="$args --network-plugin=cni --host-only-cidr=20.0.0.0/16"

	# FIXME: using '|| true' to overcome following:
        # FIXME: Error cluster status: getting status: running command: sudo systemctl is-active kubelet
        echo "Starting minikube with minikube start $args"
        minikube start $args || true

        echo "Give minikube time to come up"
        sleep 5

        echo "Get minikube status"
        minikube status

        kubectl config use-context minikube

        for i in .kube .minikube; do
                sudo rm -rf /root/$i
                sudo cp -ar $HOME/$i /root/$i
        done

	$WITH_CALICO && kubectl apply -f $CALICO_URL

        kubectl get services kubernetes
        kubectl get pods -n kube-system
}

status() {
        kubectl version
        kubectl config get-contexts
        minikube status
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
