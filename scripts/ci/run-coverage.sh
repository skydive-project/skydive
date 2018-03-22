#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

# this should deploy in the CI image
sudo yum install -y screen inotify-tools

sudo systemctl stop etcd.service
sleep 15

sudo iptables -F
sudo iptables -P FORWARD ACCEPT
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

set -e

cd ${GOPATH}/src/github.com/skydive-project/skydive

[ -d /tmp/netcleanup ] || sudo scripts/ci/cleanup.sh init
export ARGS="-graph.output ascii"
./coverage.sh --no-functionals --no-scale
sudo scripts/ci/cleanup.sh snapshot
sudo scripts/ci/cleanup.sh cleanup
./coverage.sh --no-units --no-scale
sudo scripts/ci/cleanup.sh snapshot
sudo scripts/ci/cleanup.sh cleanup
./coverage.sh --no-units --no-scale --orientdb
sudo scripts/ci/cleanup.sh snapshot
sudo scripts/ci/cleanup.sh cleanup
./coverage.sh --no-units --no-functionals --coveralls
