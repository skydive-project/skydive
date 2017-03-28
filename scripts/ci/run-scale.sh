#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go-deps.sh"

# this should deploy in the CI image
sudo yum install -y screen inotify-tools

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make install
TLS=false ELASTICSEARCH=localhost:9200 SKYDIVE=${GOPATH}/bin/skydive make test.functionals TAGS="scale test" VERBOSE=true TIMEOUT=10m TEST_PATTERN=HA
