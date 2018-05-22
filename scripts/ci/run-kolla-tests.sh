#!/bin/bash

set -v

dir="$(dirname "$0")"

make -f ${dir}/../../Makefile static
cp $GOPATH/bin/skydive .
make -f ${dir}/../../Makefile test.functionals.compile GOFLAGS="-race" GORACE="history_size=5" WITH_NEUTRON=true VERBOSE=true
cd ${dir}/kolla
DEPLOYMENT_MODE=dev vagrant up --provider=libvirt
retcode=$?
vagrant destroy --force
exit $retcode
