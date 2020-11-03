#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

. "$DIR/run-tests-utils.sh"
network_setup
WITH_EBPF=true
WITH_K8S=true
TEST_PATTERN="K8s"
tests_run
exit $RETCODE
