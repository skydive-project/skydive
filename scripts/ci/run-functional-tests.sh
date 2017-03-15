#!/bin/bash

set -v

dir="$(dirname "$0")"

. "${dir}/install-requirements.sh"
. "${dir}/install-go.sh"

GOFLAGS="-race"

case "$BACKEND" in
  "memory")
    exit 0
    ;;
  "orientdb")
    . "${dir}/install-orientdb.sh"
    cd ${ORIENTDBPATH}
    export ORIENTDB_ROOT_PASSWORD=root
    ${ORIENTDBPATH}/bin/server.sh &
    sleep 5
    ARGS="-graph.backend orientdb -storage.backend orientdb"
    GOFLAGS="$GOFLAGS"
    TAGS="storage"
    ;;
  "elasticsearch")
    . "${dir}/install-elasticsearch.sh"
    ARGS="-graph.backend elasticsearch -storage.backend elasticsearch"
    GOFLAGS="$GOFLAGS"
    TAGS="storage"
    ;;
esac

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make test.functionals.batch TAGS="$TAGS" GOFLAGS="$GOFLAGS" GORACE="history_size=5" VERBOSE=true TIMEOUT=20m ARGS="$ARGS -standalone -etcd.server http://localhost:2379" | go2xunit -output $WORKSPACE/tests.xml
