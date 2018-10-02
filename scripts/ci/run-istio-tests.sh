#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

istio_setup() {
        . "$DIR/install-minikube.sh" install
        . "$DIR/install-minikube.sh" stop
        . "$DIR/install-minikube.sh" start

        . "$DIR/install-istio.sh" install
        . "$DIR/install-istio.sh" stop
        . "$DIR/install-istio.sh" start
}

istio_teardown() {
        [ "$KEEP_RESOURCES" = "true" ] || . "$DIR/install-istio.sh" stop
        [ "$KEEP_RESOURCES" = "true" ] || . "$DIR/install-minikube.sh" stop
}

. "$DIR/run-tests-utils.sh"
network_setup
istio_setup
WITH_ISTIO=true
TEST_PATTERN='\(Istio\|K8s\)'
tests_run
istio_teardown
exit $RETCODE
