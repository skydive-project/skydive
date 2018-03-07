#!/bin/bash

set -e

cd devstack.git
source openrc admin admin
export OS_AUTH_URL=${OS_AUTH_URL}/v${OS_IDENTITY_API_VERSION}
export PATH=$PATH:/opt/go/bin:/opt/stack/go/bin:/opt/stack/protoc/bin
export GOROOT=/opt/go
export GOPATH=/opt/stack/go
export GO_VERSION=1.8
cd /opt/stack/go/src/github.com/skydive-project/skydive/
SKYDIVE_ANALYZERS=localhost:8082 make test.functionals WITH_NEUTRON=true VERBOSE=true TIMEOUT=5m TEST_PATTERN=Neutron
