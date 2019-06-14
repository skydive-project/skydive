#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

. "$DIR/run-tests-utils.sh"
network_setup
WITH_DOCKER_VPP=true WITH_OVN=true WITH_OPENCONTRAIL=false tests_run
exit $RETCODE
