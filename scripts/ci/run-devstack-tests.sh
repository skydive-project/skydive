#!/bin/bash

set -e
set -v

function vagrant_cleanup {
    vagrant destroy --force
}
[ "$KEEP_RESOURCES" = "true" ] || trap vagrant_cleanup EXIT

dir="$(dirname "$0")"
cd ${dir}/devstack
vagrant up --no-provision --provider=libvirt && vagrant provision
