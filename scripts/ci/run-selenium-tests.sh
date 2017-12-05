#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

sudo systemctl stop etcd.service
sleep 15

sudo iptables -F
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export SKYDIVE_ANALYZERS=localhost:8082
export SKYDIVE=${GOPATH}/bin/skydive

make test.functionals GOTAGS="selenium" VERBOSE=true TIMEOUT=10m TEST_PATTERN=PacketInjectionCapture ARGS="$ARGS -standalone"
