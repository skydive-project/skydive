#!/bin/bash

set -v

dir="$(dirname "$0")"
cd ${dir}/devstack
vagrant up --no-provision --provider=libvirt && vagrant provision
retcode=$?
vagrant destroy --force
exit $retcode
