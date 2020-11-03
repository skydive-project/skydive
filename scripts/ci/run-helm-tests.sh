#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

. "$DIR/run-tests-utils.sh"
network_setup
WITH_HELM=true
TEST_PATTERN=Helm
tests_run
exit $RETCODE
