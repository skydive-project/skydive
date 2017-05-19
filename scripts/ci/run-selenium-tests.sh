#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

sudo systemctl stop etcd.service
sleep 15

sudo iptables -F

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export SKYDIVE_ANALYZERS=localhost:8082
export SKYDIVE=${GOPATH}/bin/skydive

make test.functionals TAGS="selenium test" VERBOSE=true TIMEOUT=10m TEST_PATTERN=Selenium ARGS="$ARGS -standalone"
