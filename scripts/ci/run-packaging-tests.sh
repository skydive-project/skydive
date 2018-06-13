#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make rpm BOOTSTRAP_ARGS=-l
rpmlint contrib/packaging/rpm/skydive.spec
