#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

. "${dir}/install-minikube.sh" install
. "${dir}/install-minikube.sh" stop
. "${dir}/install-minikube.sh" start

sudo iptables -F
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

cd ${GOPATH}/src/github.com/skydive-project/skydive

if [ "$COVERAGE" != "true" ]; then
    GOFLAGS="-race"
fi

case "$BACKEND" in
  "orientdb")
    export ORIENTDB_ROOT_PASSWORD=root
    ARGS="-graph.backend orientdb -storage.backend orientdb"
    ;;
  "elasticsearch")
    ARGS="-graph.backend elasticsearch -storage.backend elasticsearch"
    ;;
esac

set -e
make test.functionals.batch GOFLAGS="$GOFLAGS" GORACE="history_size=5" WITH_EBPF=true WITH_K8S=true VERBOSE=true TIMEOUT=20m COVERAGE=$COVERAGE ARGS="$ARGS -graph.output ascii -standalone" TEST_PATTERN=$TEST_PATTERN 2>&1 | tee $WORKSPACE/output.log
go2xunit -fail -fail-on-race -input $WORKSPACE/output.log -output $WORKSPACE/tests.xml
set +e

. "${dir}/install-minikube.sh" stop || true

if [ -e functionals.cover ]; then
    mv functionals.cover functionals-${BACKEND}.cover
fi
