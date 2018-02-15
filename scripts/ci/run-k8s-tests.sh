#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go-deps.sh"

set -e

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export ELASTICSEARCH=localhost:9200
export SKYDIVE_ANALYZERS=localhost:8082
export SKYDIVE_LOGGING_LEVEL=DEBUG

ARGS= \
	"-standalone" \
	"-graph.backend=elasticsearch" \
	"-storage.backend=elasticsearch" \
	"-analyzer.topology.probes=k8s"

make test.functionals WITH_K8S=true VERBOSE=true TIMEOUT=1m ARGS="$ARGS"
