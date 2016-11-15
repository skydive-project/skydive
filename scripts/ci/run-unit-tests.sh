#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-go.sh"
. "${dir}/install-requirements.sh"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make test GOFLAGS=-race VERBOSE=true TIMEOUT=1m
