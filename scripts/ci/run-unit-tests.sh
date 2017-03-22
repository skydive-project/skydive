#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go-deps.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make test GOFLAGS=-race VERBOSE=true TIMEOUT=1m | go2xunit -output $WORKSPACE/tests.xml
