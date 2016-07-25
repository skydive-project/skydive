#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-go.sh"

# workaround until CI image are updated
[ -d "${GOPATH}/src/github.com/redhat-cip" ] && mv ${GOPATH}/src/github.com/{redhat-cip,skydive-project}

cd ${GOPATH}/src/github.com/skydive-project/skydive
make test GOFLAGS=-race VERBOSE=true TIMEOUT=1m
