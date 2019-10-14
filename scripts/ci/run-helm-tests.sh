#!/bin/bash

set -v
set -e

go get -f -u github.com/tebeka/go2xunit

DIR="$(dirname "$0")"

helm_setup() {
        . "$DIR/install-minikube.sh" install
        . "$DIR/install-minikube.sh" stop
        . "$DIR/install-minikube.sh" start

        . "$DIR/install-helm.sh" install
        . "$DIR/install-helm.sh" stop
        . "$DIR/install-helm.sh" start
}

helm_teardown() {
        [ "$KEEP_RESOURCES" = "true" ] || . "$DIR/install-helm.sh" stop
        [ "$KEEP_RESOURCES" = "true" ] || . "$DIR/install-minikube.sh" stop
}

. "$DIR/run-tests-utils.sh"
network_setup
helm_setup
WITH_HELM=true
TEST_PATTERN=Helm
tests_run
helm_teardown
exit $RETCODE
