#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/redhat-cip/skydive
make test GOFLAGS=-race VERBOSE=true TIMEOUT=6m
