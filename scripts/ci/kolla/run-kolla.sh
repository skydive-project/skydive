#!/bin/sh

# Install dependencies
yum -y install epel-release
yum -y install python-pip ansible python-devel libffi-devel gcc openssl-devel libselinux-python
pip install -U pip

# Install Kolla
if [ "$DEPLOYMENT_MODE" == "dev" ]; then
  git clone https://github.com/openstack/kolla
  git clone https://github.com/openstack/kolla-ansible
  pip install -U 'oslo.config==5.2.0'
  pip install -r kolla-ansible/requirements.txt
  pip install -r kolla/requirements.txt

  cp -r kolla-ansible/etc/kolla /etc/kolla/
  cp kolla-ansible/ansible/inventory/all-in-one .

  # Generate Kolla password
  kolla-ansible/tools/generate_passwords.py

  KOLLA_ANSIBLE=`pwd`/kolla-ansible/tools/kolla-ansible
else
  pip install kolla-ansible
  cp -r /usr/share/kolla-ansible/etc_examples/kolla /etc/kolla/
  cp /usr/share/kolla-ansible/ansible/inventory/all-in-one .

  # Generate Kolla passwords
  kolla-genpwd

  KOLLA_ANSIBLE=kolla-ansible
fi

# Modify globals.yml
cat >> /etc/kolla/globals.yml <<EOF
kolla_base_distro: "centos"
kolla_install_type: "binary"
openstack_release: "master"
kolla_internal_vip_address: "192.168.50.210"
network_interface: "eth1"
neutron_external_interface: "eth2"
neutron_plugin_agent: "openvswitch"
enable_fluentd: "no"
enable_heat: "no"
enable_horizon: "no"
enable_skydive: "yes"
skydive_analyzer_image: "192.168.50.200:4000/kolla/centos-binary-skydive-analyzer"
skydive_analyzer_tag: "devel"
skydive_analyzer_image_full: "192.168.50.200:4000/kolla/centos-binary-skydive-analyzer:devel"
skydive_agent_image: "192.168.50.200:4000/kolla/centos-binary-skydive-agent"
skydive_agent_tag: "devel"
skydive_agent_image_full: "192.168.50.200:4000/kolla/centos-binary-skydive-agent:devel"
EOF

$KOLLA_ANSIBLE -i ./all-in-one bootstrap-servers

# Allow access to local registry
sed -i 's|/usr/bin/dockerd|/usr/bin/dockerd --insecure-registry 192.168.50.200:4000|' /etc/systemd/system/docker.service.d/kolla.conf
systemctl daemon-reload
systemctl restart docker

# Start the local registry
docker run -d -p 4000:5000 --restart=always --name registry registry:2

# Create a simple server to provide the Skydive binary through HTTP
cd /skydive
python -m SimpleHTTPServer 8888 &
cd -

# Modify the Skydive binary URL and build the images
cd kolla
sed -i 's|https://github.com/skydive-project/skydive/releases/download/\(.*\)/skydive|http://192.168.50.200:8888/skydive|' docker/skydive/skydive-base/Dockerfile.j2
tools/build.py --registry 192.168.50.200:4000 --push -b centos skydive-agent --tag devel
tools/build.py --registry 192.168.50.200:4000 --push -b centos skydive-analyzer --tag devel
cd -

# Deploy Kolla
$KOLLA_ANSIBLE -i ./all-in-one prechecks
$KOLLA_ANSIBLE -i ./all-in-one deploy
$KOLLA_ANSIBLE -i ./all-in-one post-deploy
