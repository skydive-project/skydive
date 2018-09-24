#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

istio_setup() {
        . "$DIR/install-minikube.sh" install
        . "$DIR/install-minikube.sh" stop || true
        . "$DIR/install-minikube.sh" start

        . "$DIR/install-istio.sh" install
        . "$DIR/install-istio.sh" stop || true
        . "$DIR/install-istio.sh" start
}

istio_teardown() {
        . "$DIR/install-istio.sh" stop || true
        . "$DIR/install-minikube.sh" stop || true
}

[ "$KEEP_RESOURCES" = "true" ] || trap istio_teardown EXIT

export no_proxy=$no_proxy,192.168.99.100

. "$DIR/run-tests-utils.sh"
network_setup
istio_setup
WITH_K8S=true
WITH_ISTIO=true
TEST_PATTERN='\(Istio\|K8s\)'
tests_run
exit $RETCODE
