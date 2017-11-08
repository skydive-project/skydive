#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

# this should deploy in the CI image
sudo yum install -y screen inotify-tools iperf

sudo systemctl stop etcd.service
sleep 15

sudo iptables -F
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export SKYDIVE_ANALYZERS=localhost:8082
export ELASTICSEARCH=localhost:9200
export TLS=true
export SKYDIVE=${GOPATH}/bin/skydive
export FLOW_PROTOCOL=websocket
export SKYDIVE_LOGGING_LEVEL=DEBUG

make test.functionals TAGS="scale" VERBOSE=true TIMEOUT=10m TEST_PATTERN=Scale
status=$?

if [ $status -ne 0 ] ; then
    cat /tmp/skydive-scale/{analyzer,agent}-?.log | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | perl -ne '$d=$1 if /^(\d+-\d+-\d+),/; $k{$d}.=$_; END{print $k{$_} for sort keys(%k);}'
    echo "test Scale TLS:${TLS} FLOW_PROTOCOL:${FLOW_PROTOCOL} failed return ${status}"
    exit $status
fi

# clean elasticsearch
curl -X 'DELETE http://localhost:9200/skydive*'

export FLOW_PROTOCOL=udp
make test.functionals TAGS="scale test" VERBOSE=true TIMEOUT=15m FLOW_PROTOCOL=udp TEST_PATTERN=Scale
status=$?

if [ $status -ne 0 ] ; then
    cat /tmp/skydive-scale/{analyzer,agent}-?.log | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | perl -ne '$d=$1 if /^(\d+-\d+-\d+),/; $k{$d}.=$_; END{print $k{$_} for sort keys(%k);}'
    echo "test Scale TLS:${TLS} FLOW_PROTOCOL:${FLOW_PROTOCOL} failed return ${status}"
    exit $status
fi

exit 0
