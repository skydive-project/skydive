#!/bin/bash

set -v

dir="$(dirname "$0")"

set -e

cd ${GOPATH}/src/github.com/skydive-project/skydive

make fmt
make vet
make check
