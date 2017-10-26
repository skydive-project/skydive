#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make WITH_DPDK=true VERBOSE=true
