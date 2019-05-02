#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive

make govendor
d=$(git diff)
if [ $(echo -n $d | wc -l) -ne 0 ] ; then
    echo -e "ERROR FAIL: govendor is not synched correctly\n\n$d"
    exit 1
fi

# prepare collectd build
rm -rf /tmp/collectd
git clone https://github.com/collectd/collectd.git /tmp/collectd

# Compile all contribs
COLLECTD_SRC=/tmp/collectd make contribs

# Compile with default options
make
make test.functionals.compile TAGS=${TAGS}

# Compile with all build options supported enabled
make WITH_DPDK=true WITH_EBPF=true WITH_VPP=true WITH_EBPF_DOCKER_BUILDER=true WITH_K8S=true WITH_ISTIO=true WITH_HELM=true VERBOSE=true

# Compile Skydive for Windows
GOOS=windows GOARCH=amd64 govendor build github.com/skydive-project/skydive

# Compile Skydive for MacOS
GOOS=darwin GOARCH=amd64 govendor build github.com/skydive-project/skydive

# Compile profiling
make WITH_PROF=true VERBOSE=true

# Compile all tests
make test.functionals.compile TAGS=${TAGS} WITH_NEUTRON=true WITH_SELENIUM=true WITH_CDD=true WITH_SCALE=true WITH_EBPF=true WITH_VPP=true WITH_K8S=true WITH_ISTIO=true WITH_HELM=true WITH_EBPF_DOCKER_BUILDER=true

# Compile static
make static
