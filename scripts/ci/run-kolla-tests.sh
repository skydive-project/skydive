#!/bin/bash

set -e
set -v

dir="$(dirname "$0")"

function vagrant_cleanup {
    vagrant destroy --force
}
trap vagrant_cleanup EXIT

make -f ${dir}/../../Makefile static
cp $GOPATH/bin/skydive .
make -f ${dir}/../../Makefile test.functionals.compile GOFLAGS="-race" TAGS="${TAGS}" GORACE="history_size=5" WITH_NEUTRON=true VERBOSE=true
cd ${dir}/kolla
DEPLOYMENT_MODE=dev vagrant up --provider=libvirt
vagrant destroy --force
