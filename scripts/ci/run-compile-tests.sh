#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive

# prepare collectd build
rm -rf /tmp/collectd
git clone https://github.com/collectd/collectd.git /tmp/collectd

# Compile collectd plugin
rm -rf ../skydive-collectd-plugin
git clone https://github.com/skydive-project/skydive-collectd-plugin ../skydive-collectd-plugin
echo "replace github.com/skydive-project/skydive => ../skydive" >> ../skydive-collectd-plugin/go.mod
COLLECTD_SRC=/tmp/collectd SKYDIVE_GO_MOD=${GOPATH}/src/github.com/skydive-project/skydive/go.mod make -C ../skydive-collectd-plugin

# Compile all contribs
make contribs

# Compile with default options
make
make test.functionals.compile TAGS=${TAGS}

export CGO_CFLAGS="-I/usr/local/include/dpdk -O3 -g -std=gnu11 -m64 -pthread -march=native -DRTE_MACHINE_CPUFLAG_SSE -DRTE_MACHINE_CPUFLAG_SSE2 -DRTE_MACHINE_CPUFLAG_SSE3 -DRTE_MACHINE_CPUFLAG_SSSE3 -DRTE_MACHINE_CPUFLAG_SSE4_1 -DRTE_MACHINE_CPUFLAG_SSE4_2 -DRTE_MACHINE_CPUFLAG_PCLMULQDQ -DRTE_MACHINE_CPUFLAG_RDRAND -DRTE_MACHINE_CPUFLAG_FSGSBASE -DRTE_MACHINE_CPUFLAG_F16C -include rte_config.h -Wno-deprecated-declarations"
export CGO_LDFLAGS="-L/usr/local/lib"

# Compile with all build options supported enabled
make WITH_DPDK=true WITH_EBPF=true WITH_VPP=true WITH_EBPF_DOCKER_BUILDER=true WITH_K8S=true WITH_ISTIO=true \
    WITH_HELM=true VERBOSE=true

# Compile Skydive for Windows
GOOS=windows GOARCH=amd64 go build github.com/skydive-project/skydive

# Compile Skydive for MacOS
GOOS=darwin GOARCH=amd64 go build github.com/skydive-project/skydive

# Compile profiling
make WITH_PROF=true VERBOSE=true

make clean

# Compile all tests
make test.functionals.compile TAGS=${TAGS} WITH_NEUTRON=true WITH_SELENIUM=true WITH_CDD=true \
    WITH_SCALE=true WITH_EBPF=true WITH_EBPF_DOCKER_BUILDER=true WITH_VPP=true WITH_K8S=true \
    WITH_ISTIO=true WITH_HELM=true

make clean

# Compile static
make static
