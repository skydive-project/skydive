#!/bin/bash

set -v

dir="$(dirname "$0")"

sudo iptables -F
sudo iptables -P FORWARD ACCEPT
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export SKYDIVE_ANALYZERS=localhost:8082
export ELASTICSEARCH=localhost:9200
export TLS=true
export SKYDIVE=${GOPATH}/bin/skydive
export FLOW_PROTOCOL=${FLOW_PROTOCOL:-udp}
export SKYDIVE_LOGGING_LEVEL=DEBUG

make test.functionals WITH_SCALE=true VERBOSE=true TIMEOUT=10m TEST_PATTERN=Scale
status=$?

cat /tmp/skydive-scale/{analyzer,agent}-?.log | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | perl -ne '$d=$1 if /^(\d+-\d+-\d+),/; $k{$d}.=$_; END{print $k{$_} for sort keys(%k);}'
if [ $status -ne 0 ] ; then
   echo "test Scale TLS:${TLS} FLOW_PROTOCOL:${FLOW_PROTOCOL} failed return ${status}"
   exit $status
fi

exit 0
