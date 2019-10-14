#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

. "$DIR/run-tests-utils.sh"
network_setup

# Build the skydive executable for starting seeds
make WITH_VPP=true

WITH_OVN=true WITH_OPENCONTRAIL=false WITH_EBPF=true WITH_VPP=true \
SKYDIVE_SEED_EXECUTABLE=$GOPATH/bin/skydive \
BACKEND=elasticsearch TEST_PATTERN='(EBPF|SRIOV|VPP|Libvirt)' \
TAGS="$TAGS libvirt_tests sriov_tests" tests_run

exit $RETCODE
