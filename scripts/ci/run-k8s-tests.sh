#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

k8s_setup() {
        . "$DIR/install-minikube.sh" install
        . "$DIR/install-minikube.sh" start
}

k8s_teardown() {
        [ "$KEEP_RESOURCES" = "true" ] || . "$DIR/install-minikube.sh" stop
}

. "$DIR/run-tests-utils.sh"
k8s_setup
network_setup
WITH_EBPF=true
WITH_K8S=true
TEST_PATTERN="K8s"
tests_run
k8s_teardown
exit $RETCODE
