#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-requirements.sh"
. "${dir}/install-go.sh"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make test GOFLAGS=-race VERBOSE=true TIMEOUT=1m
