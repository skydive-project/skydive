#!/bin/sh

# Must be provided by Jenkins credentials plugin:
# VAGRANTCLOUD_TOKEN

if [ -z "$VAGRANTCLOUD_TOKEN" ]
then
    echo "The environment variable VAGRANTCLOUD_TOKEN needs to be defined"
    exit 1
fi

dir="$(dirname "$0")"

# SKYDIVE_RELEASE is a relase tag (like "v0.1.2") or "master"
export SKYDIVE_RELEASE=master
BOXVERSION=1.0.0
tagname=$(git show-ref --tags $REF)
if [ -n "$tagname" ]; then
    export SKYDIVE_RELEASE=$(echo $tagname | awk -F "/" "{print \$NF}")
    BOXVERSION=$(echo $SKYDIVE_RELEASE | tr -d '[a-z]')
fi

cd ${dir}/../../contrib/dev

vagrant plugin install vagrant-reload

function vagrant_cleanup {
    vagrant destroy --force
}
[ "$KEEP_RESOURCES" = "true" ] || trap vagrant_cleanup EXIT

set -v
set -e

[ -z "$PROVIDERS" ] && PROVIDERS="libvirt virtualbox"
for provider in $PROVIDERS
do
    [ "$provider" = "virtualbox" ] && export PRIVATE_IP=192.168.99.10
    PREPARE_BOX=true vagrant up --provider=$provider ${KEEP_RESOURCES:+--no-destroy-on-error}
    [ "$provider" = "libvirt" ] && sudo chmod a+r /var/lib/libvirt/images/dev_dev.img || true

    # skydive testing
    vagrant ssh -c 'set -e; cd go/src/github.com/skydive-project/skydive; make test functional; curl -XDELETE "localhost:9200/skydive*"'
    
    vagrant package --out skydive-dev-$provider.box
    vagrant destroy --force

    json=`curl "https://vagrantcloud.com/api/v1/box/skydive/skydive-dev/version/$BOXVERSION/provider/$provider/upload?access_token=$VAGRANTCLOUD_TOKEN"`
    upload_path=`echo $json | jq .upload_path | cut -d '"' -f 2`

    if [ -n "$DRY_RUN" ]; then
        echo "Running in dry run mode. Skipping upload."
    else
        curl -X PUT --upload-file skydive-dev-$provider.box $upload_path
    fi
done
