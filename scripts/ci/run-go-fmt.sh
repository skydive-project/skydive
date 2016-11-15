#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-go.sh"
. "${dir}/install-requirements.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

make fmt
