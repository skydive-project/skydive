#!/bin/bash

set -v

dir="$(dirname "$0")"

go get -f -u github.com/golang/lint/golint

set -e

cd ${GOPATH}/src/github.com/skydive-project/skydive

make fmt
make vet
make check