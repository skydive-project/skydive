#!/bin/bash

set +x
set -e

SKYDIVE_PATH=$PWD

QUICKSTART=${QUICKSTART:-/tmp/tripleo-quickstart}
NODES=${NODE:-$QUICKSTART/config/nodes/1ctlr_1comp.yml}
CONFIG=${CONFIG:-$SKYDIVE_PATH/scripts/ci/tripleo-quickstart/minimal.yml}
VHOST=${VHOST:-127.0.0.2}
SKYDIVE_CONFIG=${SKYDIVE_CONFIG:-scripts/ci/tripleo-quickstart/skydive-minimal.yaml}

sudo rm -rf ~/.quickstart/
sudo rm -rf /tmp/tripleo-quickstart
git clone https://github.com/openstack/tripleo-quickstart.git /tmp/tripleo-quickstart

sed -i -e 's/retries = 3/retries = 10/' /tmp/tripleo-quickstart/ansible.cfg

pushd $QUICKSTART
bash quickstart.sh -R master --no-clone --tags all \
	--requirements quickstart-extras-requirements.txt \
	--nodes $NODES --config $CONFIG -p quickstart.yml $VHOST

bash quickstart.sh -R master --no-clone --tags all --nodes $NODES \
        --config $CONFIG \
	-I --teardown none -p quickstart-extras-undercloud.yml $VHOST
popd

scp -F ~/.quickstart/ssh.config.ansible -r ../skydive undercloud:

ssh -F ~/.quickstart/ssh.config.ansible undercloud "sudo ln -s /home/stack/skydive/contrib/ansible /usr/share/skydive-ansible"

scp -F ~/.quickstart/ssh.config.ansible $SKYDIVE_CONFIG undercloud:skydive.yaml

pushd $QUICKSTART
bash quickstart.sh -R master --no-clone --tags all --nodes $NODES \
	--config $CONFIG \
	-I --teardown none -p quickstart-extras-overcloud-prep.yml $VHOST

bash quickstart.sh -R master --no-clone --tags all --nodes $NODES \
	--config $CONFIG \
	-I --teardown none -p quickstart-extras-overcloud.yml $VHOST
popd

scp -F ~/.quickstart/ssh.config.ansible $SKYDIVE_PATH/scripts/ci/tripleo-tests.sh undercloud:
ssh -F ~/.quickstart/ssh.config.ansible undercloud "bash -x tripleo-tests.sh"
