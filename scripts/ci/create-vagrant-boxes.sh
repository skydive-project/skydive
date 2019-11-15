#!/bin/sh

# Must be provided by Jenkins credentials plugin:
# VAGRANTCLOUD_TOKEN

if [ -z "$VAGRANTCLOUD_TOKEN" ]
then
    echo "The environment variable VAGRANTCLOUD_TOKEN needs to be defined"
    exit 1
fi

dir="$(dirname "$0")"

# SKYDIVE_RELEASE is a release tag (like "v0.1.2") or "master"
export SKYDIVE_RELEASE=master
BOXVERSION=0.27.0.alpha
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
    MEMORY=8192 PREPARE_BOX=true SKYDIVE_SYNC_FOLDER=true vagrant up --provider=$provider ${KEEP_RESOURCES:+--no-destroy-on-error}
    [ "$provider" = "libvirt" ] && sudo chmod a+r /var/lib/libvirt/images/dev_dev.img || true

    echo "Running Skydive tests"
    vagrant ssh -c 'set -e; cd go/src/github.com/skydive-project/skydive; make test functional WITH_OVN=true; curl -XDELETE "localhost:9200/skydive*"'
    
    vagrant package --out skydive-dev-$provider.box
    vagrant destroy --force

    if [ -n "$DRY_RUN" ]; then
        echo "Running in dry run mode. Skipping upload."
    else
        # Create a new version
        curl \
        --header "Content-Type: application/json" \
        --header "Authorization: Bearer $VAGRANTCLOUD_TOKEN" \
        https://app.vagrantup.com/api/v1/box/skydive/skydive-dev/versions \
        --data "{ \"version\": { \"version\": \"$BOXVERSION\" } }" || true

        # Create libvirt provider
        curl \
        --header "Content-Type: application/json" \
        --header "Authorization: Bearer $VAGRANTCLOUD_TOKEN" \
        https://app.vagrantup.com/api/v1/box/skydive/skydive-dev/version/$BOXVERSION/providers \
        --data '{ "provider": { "name": "libvirt" } }' || true

        # Create virtualbox provider
        curl \
        --header "Content-Type: application/json" \
        --header "Authorization: Bearer $VAGRANTCLOUD_TOKEN" \
        https://app.vagrantup.com/api/v1/box/skydive/skydive-dev/version/$BOXVERSION/providers \
        --data '{ "provider": { "name": "virtualbox" } }' || true

        # Get the upload URL
        json=`curl "https://app.vagrantup.com/api/v1/box/skydive/skydive-dev/version/$BOXVERSION/provider/$provider/upload?access_token=$VAGRANTCLOUD_TOKEN"`
        upload_path=`echo $json | jq .upload_path | cut -d '"' -f 2`

        # Upload the file
        curl -X PUT --upload-file skydive-dev-$provider.box $upload_path
    fi
done
