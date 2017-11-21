#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make rpm BOOTSTRAP_ARGS=-l
rpmlint contrib/packaging/rpm/skydive.spec
