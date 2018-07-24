#!/bin/bash

set +x
set -e

SKYDIVE_PATH=$PWD

pushd ${GOPATH}/src/github.com/skydive-project/skydive
make static
popd

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

scp -F ~/.quickstart/ssh.config.ansible -r ../skydive undercloud:skydive.git

scp -F ~/.quickstart/ssh.config.ansible -r ${GOPATH}/bin/skydive undercloud:

ssh -F ~/.quickstart/ssh.config.ansible undercloud "sudo ln -s /home/stack/skydive.git/contrib/ansible /usr/share/skydive-ansible"

scp -F ~/.quickstart/ssh.config.ansible $SKYDIVE_CONFIG undercloud:skydive.yaml

pushd $QUICKSTART
bash quickstart.sh -R master --no-clone --tags all --nodes $NODES \
	--config $CONFIG \
	-I --teardown none -p quickstart-extras-overcloud-prep.yml $VHOST

ssh -F ~/.quickstart/ssh.config.ansible undercloud <<'EOF'
REGISTRY=$(grep push_destination containers-prepare-parameter.yaml | head -n 1 | awk '{print $3}' | tr -d '"' )

sudo iptables -I INPUT -p tcp --dport 18888 -j ACCEPT
python -m SimpleHTTPServer 18888 &

rm -rf kolla
git clone https://github.com/openstack/kolla

pushd kolla
sed -i "s|https://github.com/skydive-project/skydive/releases/download/\(.*\)/skydive|http://172.17.0.1:18888/skydive|" docker/skydive/skydive-base/Dockerfile.j2
tools/build.py --registry $REGISTRY --push -b centos skydive-agent --tag devel
tools/build.py --registry $REGISTRY --push -b centos skydive-analyzer --tag devel
popd

echo "  DockerSkydiveAnalyzerImage: $REGISTRY/kolla/centos-binary-skydive-agent" >> skydive.yaml
echo "  DockerSkydiveAgentImage: $REGISTRY/kolla/centos-binary-skydive-agent" >> skydive.yaml

exit
EOF

bash quickstart.sh -R master --no-clone --tags all --nodes $NODES \
	--config $CONFIG \
	-I --teardown none -p quickstart-extras-overcloud.yml $VHOST
popd

ssh -F ~/.quickstart/ssh.config.ansible undercloud "bash -x skydive.git/scripts/ci/tripleo-tests.sh"
