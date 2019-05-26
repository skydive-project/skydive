#!/bin/bash

set -v
set -e

DIR="$(dirname "$0")"

. "$DIR/run-tests-utils.sh"
network_setup
WITH_SA=true
TEST_PATTERN="SecurityAdvisor"
tests_run
exit $RETCODE
