#!/bin/bash

DIR=$(dirname $0)

OS=linux
TARGET_DIR=/usr/bin

ISTIO_VERSION="1.0.1"
ISTIO_URL="https://github.com/istio/istio/releases/download/$ISTIO_VERSION/istio-$ISTIO_VERSION-$OS.tar.gz"
ISTIO_PKG="istio-$ISTIO_VERSION"
ISTIO_OBJECTS="$TARGET_DIR/$ISTIO_PKG/install/kubernetes/istio-demo.yaml"
ISTIO_NS=istio-system

uninstall_istio() {
        sudo rm -rf $TARGET_DIR/$ISTIO_PKG
        sudo rm -rf $TARGET_DIR/istioctl
}

install_istio() {
        local tmpdir=$(mktemp -d)
        cd $tmpdir
        curl -L "$ISTIO_URL" | tar xz
        if [ $? != 0 ]; then
                echo "failed to download $url"
                rm -rf $tmpdir
                exit 1
        fi

        chmod a+x $ISTIO_PKG
        sudo cp $ISTIO_PKG/bin/istioctl $TARGET_DIR/.
        sudo mv $ISTIO_PKG $TARGET_DIR/.
        rm -rf $tmpdir
}

check_istio() {
        which istioctl 2>/dev/null
        if [ $? != 0 ]; then
                echo "istioctl is not installed. Please run install-istio.sh install"
                exit 1
        fi
}

install() {
        install_istio
}

uninstall() {
        uninstall_istio
}

stop() {
        check_istio
        kubectl delete -f $ISTIO_OBJECTS
}

start() {
        check_istio
        kubectl apply -f $ISTIO_OBJECTS
        status
        kubectl -n $ISTIO_NS get services
}

status() {
        istioctl version
        # TODO: istio status - should be filled
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
