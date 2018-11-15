#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive

# Compile with DPDK supported enabled
make WITH_DPDK=true VERBOSE=true

# Compile Skydive for Windows
GOOS=windows GOARCH=amd64 govendor build github.com/skydive-project/skydive

# Compile Skydive for MacOS
GOOS=darwin GOARCH=amd64 govendor build github.com/skydive-project/skydive

# Compile profiling
make WITH_PROF=true VERBOSE=true

# Compile all tests
make test.functionals.compile TAGS=${TAGS} WITH_NEUTRON=true WITH_SELENIUM=true WITH_CDD=true WITH_SCALE=true

# Compile static
make static
