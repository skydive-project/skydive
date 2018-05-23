#!/bin/sh

# Must be provided by Jenkins credentials plugin:
# VAGRANTCLOUD_TOKEN

if [ -z "$VAGRANTCLOUD_TOKEN" ]
then
    echo "The environment variable VAGRANTCLOUD_TOKEN needs to be defined"
    exit 1
fi

dir="$(dirname "$0")"

VERSION="$(grep 'skydive_release:' ${dir}/../../contrib/ansible/roles/skydive_common/defaults/main.yml | cut -f 2 -d ' ' | tr -d 'v')"
cd ${dir}/../../contrib/dev

vagrant plugin install vagrant-openstack
vagrant plugin install vagrant-reload

set -v
set -e

for provider in libvirt virtualbox
do
    PREPARE_BOX=true vagrant up --provider=$provider
    [ "$provider" = "libvirt" ] && sudo chmod a+r /var/lib/libvirt/images/dev_dev.img || true

    vagrant package --out skydive-dev-$provider.box
    vagrant destroy --force

    json=`curl "https://vagrantcloud.com/api/v1/box/skydive/skydive-dev/version/$VERSION/provider/$provider/upload?access_token=$VAGRANTCLOUD_TOKEN"`
    upload_path=`echo $json | jq .upload_path | cut -d '"' -f 2`

    if [ -n "$DRY_RUN" ]; then
        echo "Running in dry run mode. Skipping upload."
    else
        curl -X PUT --upload-file skydive-dev-$provider.box $upload_path
    fi
done
