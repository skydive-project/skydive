#!/bin/bash

set -v

dir="$(dirname "$0")"

sudo iptables -F
sudo iptables -P FORWARD ACCEPT
for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

export ELASTICSEARCH=localhost:9200
export SKYDIVE=${GOPATH}/bin/skydive
export SKYDIVE_LOGGING_LEVEL=DEBUG

make test.functionals WITH_CDD=true TAGS=${TAGS} VERBOSE=true TIMEOUT=10m TEST_PATTERN=Overview

"${dir}/convert-to-gif.sh" tests/overview.mp4 tests/overview-tmp.gif
gifsicle -O3 tests/overview-tmp.gif -o tests/overview.gif
