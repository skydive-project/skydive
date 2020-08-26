#!/bin/bash

set -v

DIR="$(dirname "$0")"

sudo iptables -F
sudo iptables -P FORWARD ACCEPT
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export SKYDIVE_ANALYZERS=localhost:8082
export ELASTICSEARCH=localhost:9201
export TLS=true
export SKYDIVE=${GOPATH}/bin/skydive
export FLOW_PROTOCOL=${FLOW_PROTOCOL:-websocket}
export SKYDIVE_LOGGING_LEVEL=DEBUG

. "$DIR/run-tests-utils.sh"
es_setup && trap es_cleanup EXIT

make test.functionals WITH_SCALE=true TAGS=${TAGS} VERBOSE=true TIMEOUT=10m TEST_PATTERN=Scale EXTRA_ARGS="-logs=/tmp/skydive-scale/scale.log"
status=$?

sudo cat /tmp/skydive-scale/scale.log

if [ $status -ne 0 ] ; then
   echo "test Scale TLS:${TLS} FLOW_PROTOCOL:${FLOW_PROTOCOL} failed return ${status}"
   exit $status
fi

exit 0
