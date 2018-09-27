#!/bin/bash

set -v
set -eo pipefail

dir="$(dirname "$0")"

go get -f -u github.com/tebeka/go2xunit

GOFLAGS=-race
if [ "$(uname -m)" = "ppc64le" ] ; then
    GOFLAGS=""
fi

cd ${GOPATH}/src/github.com/skydive-project/skydive
make test GOFLAGS="${GOFLAGS}" TAGS="${TAGS}" VERBOSE=true TIMEOUT=5m COVERAGE=${COVERAGE} | tee $WORKSPACE/unit-tests.log
go2xunit -input $WORKSPACE/unit-tests.log -output $WORKSPACE/tests.xml
sed -i 's/\x1b\[[0-9;]*m//g' $WORKSPACE/tests.xml

# Run Benchmark
make bench.flow
