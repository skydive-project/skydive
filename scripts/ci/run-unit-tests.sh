#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make test GOFLAGS=-race VERBOSE=true TIMEOUT=1m COVERAGE=${COVERAGE} | tee $WORKSPACE/unit-tests.log
go2xunit -input $WORKSPACE/unit-tests.log -output $WORKSPACE/tests.xml
