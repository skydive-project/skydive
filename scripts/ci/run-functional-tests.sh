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
esac

cd ${GOPATH}/src/github.com/redhat-cip/skydive
make test.functionals GOFLAGS=-race VERBOSE=true TIMEOUT=6m ARGS="$ARGS"
