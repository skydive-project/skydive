#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go-deps.sh"

GOFLAGS="-race"

case "$BACKEND" in
  "orientdb")
    cd ${ORIENTDBPATH}
    export ORIENTDB_ROOT_PASSWORD=root
    ARGS="-graph.backend orientdb -storage.backend orientdb"
    GOFLAGS="$GOFLAGS"
    TAGS="storage"
    ;;
  "elasticsearch")
    ARGS="-graph.backend elasticsearch -storage.backend elasticsearch"
    GOFLAGS="$GOFLAGS"
    TAGS="storage"
    ;;
esac

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make test.functionals.batch TAGS="$TAGS" GOFLAGS="$GOFLAGS" GORACE="history_size=5" VERBOSE=true TIMEOUT=20m ARGS="$ARGS -standalone -etcd.server http://localhost:2379" 2>&1 | tee $WORKSPACE/output.log
go2xunit -fail -fail-on-race -input $WORKSPACE/output.log -output $WORKSPACE/tests.xml
