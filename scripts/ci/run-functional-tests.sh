#!/bin/bash

set -v

dir="$(dirname "$0")"

. "${dir}/install-go.sh"
. "${dir}/install-requirements.sh"

case "$BACKEND" in
  "gremlin-ws")
    . "${dir}/install-gremlin.sh"
    cd ${GREMLINPATH}
    ${GREMLINPATH}/bin/gremlin-server.sh ${GREMLINPATH}/conf/gremlin-server.yaml &
    sleep 5
    ARGS="-graph.backend gremlin-ws"
    ;;
  "gremlin-rest")
    . "${dir}/install-gremlin.sh"
    cd ${GREMLINPATH}
    ${GREMLINPATH}/bin/gremlin-server.sh ${GREMLINPATH}/conf/gremlin-server-rest-modern.yaml &
    sleep 5
    ARGS="-graph.backend gremlin-rest"
    ;;
  "orientdb")
    . "${dir}/install-orientdb.sh"
    cd ${ORIENTDBPATH}
    export ORIENTDB_ROOT_PASSWORD=root
    ${ORIENTDBPATH}/bin/server.sh &
    sleep 5
    ARGS="-graph.backend orientdb"
    ;;
esac

cd ${GOPATH}/src/github.com/skydive-project/skydive
make test.functionals GOFLAGS=-race GORACE="history_size=5" VERBOSE=true TIMEOUT=2m ARGS="$ARGS -etcd.server http://localhost:2379"
