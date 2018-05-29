#!/bin/bash

set -e
set -v

function vagrant_cleanup {
    vagrant destroy --force
}
trap vagrant_cleanup EXIT

dir="$(dirname "$0")"
cd ${dir}/devstack
vagrant up --no-provision --provider=libvirt && vagrant provision
vagrant destroy --force
