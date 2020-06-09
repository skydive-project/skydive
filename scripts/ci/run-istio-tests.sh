#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

export no_proxy=$no_proxy,192.168.99.100

. "$DIR/run-tests-utils.sh"
network_setup
WITH_K8S=true
WITH_ISTIO=true
TEST_PATTERN='(Istio|K8s)'
tests_run
exit $RETCODE
