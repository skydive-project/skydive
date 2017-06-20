#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

if [ "$COVERAGE" != "true" ]; then
    GOFLAGS="-race"
fi

case "$BACKEND" in
  "orientdb")
    export ORIENTDB_ROOT_PASSWORD=root
    ARGS="-graph.backend orientdb -storage.backend orientdb"
    TAGS="storage"
    ;;
  "elasticsearch")
    ARGS="-graph.backend elasticsearch -storage.backend elasticsearch"
    TAGS="storage"
    ;;
esac

set -e
make test.functionals.batch TAGS="$TAGS" GOFLAGS="$GOFLAGS" GORACE="history_size=5" VERBOSE=true TIMEOUT=20m COVERAGE=$COVERAGE ARGS="$ARGS -graph.output ascii -standalone -etcd.server http://localhost:2379" TEST_PATTERN=$TEST_PATTERN 2>&1 | tee $WORKSPACE/output.log
go2xunit -fail -fail-on-race -input $WORKSPACE/output.log -output $WORKSPACE/tests.xml
set +e

if [ -e functionals.cover ]; then
    mv functionals.cover functionals-${BACKEND}.cover
fi
