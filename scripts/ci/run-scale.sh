#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

# this should deploy in the CI image
sudo yum install -y screen inotify-tools

sudo systemctl stop etcd.service
sleep 15

sudo iptables -F

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export SKYDIVE_ANALYZERS=localhost:8082
export ELASTICSEARCH=localhost:9200
export TLS=true
export SKYDIVE=${GOPATH}/bin/skydive

make test.functionals TAGS="scale test" VERBOSE=true TIMEOUT=10m TEST_PATTERN=HA
status=$?

cat /tmp/skydive-scale/{analyzer,agent}-?.log | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | perl -ne '$d=$1 if /^(\d+-\d+-\d+),/; $k{$d}.=$_; END{print $k{$_} for sort keys(%k);}'

exit $status
