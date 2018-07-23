#!/bin/bash

set -e
set -v

dir="$(dirname "$0")"

function vagrant_cleanup {
    vagrant destroy --force
}
[ "$KEEP_RESOURCES" = "true" ] || trap vagrant_cleanup EXIT

make -f ${dir}/../../Makefile static
cp $GOPATH/bin/skydive .
GOFLAGS=-race
if [ "$(uname -m)" = "ppc64le" ] ; then
    GOFLAGS=""
fi
make -f ${dir}/../../Makefile test.functionals.compile GOFLAGS="${GOFLAGS}" TAGS="${TAGS}" GORACE="history_size=5" WITH_NEUTRON=true VERBOSE=true
cd ${dir}/kolla
DEPLOYMENT_MODE=dev vagrant up --provider=libvirt
vagrant destroy --force
