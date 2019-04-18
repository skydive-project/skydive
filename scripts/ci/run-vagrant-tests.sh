#!/bin/bash

if [ -n "$(sudo virt-what)" ]; then
    echo "This test must running on baremetal host"
    exit 1
fi

[ -z "$MODES" ] && MODES="binary package container"

set -e
set -v

dir="$(dirname "$0")"

root=${GOPATH}/src/github.com/skydive-project/skydive
cd $root

if [ "$DEVMODE" = "true" ]; then
    make

    make srpm
    VERSION=$(make -s version | cut -d '-' -f 1)
    TAG=$(make -s version | cut -s -d '-' -f 2- | tr '-' '.')
    mock -r epel-7-x86_64 -D "fullver $(make -s version)" --resultdir . --rebuild rpmbuild/SRPMS/skydive-${VERSION}*${TAG}.src.rpm
    mkdir -p rpmbuild/RPMS/{x86_64,noarch}
    mv skydive*-${VERSION}-*${TAG}*.x86_64.rpm rpmbuild/RPMS/x86_64/
    mv skydive*-${VERSION}-*${TAG}*.noarch.rpm rpmbuild/RPMS/noarch/

    make docker-image
    docker save skydive/skydive:devel -o skydive-docker-devel.tar
fi

cd contrib/vagrant

export ANALYZER_COUNT=1
export AGENT_COUNT=1
export SKYDIVE_RELEASE=master

function vagrant_cleanup {
    set +e
    echo "===== journalctl agent1"
    vagrant ssh agent1 -c 'sudo journalctl -xe | grep skydive'
    echo "===== journalctl analyzer1"
    vagrant ssh analyzer1 -c 'sudo journalctl -xe | grep skydive'
    vagrant destroy --force
}
[ "$KEEP_RESOURCES" = "true" ] || trap vagrant_cleanup EXIT

function install_skydive_selinux_enforcing {
    cat <<'EOF' | vagrant ssh $1 -- bash -
sudo setenforce 1
sudo sed -i 's/SELINUX=permissive/SELINUX=enforcing/' /etc/sysconfig/selinux
EOF
}

function install_skydive_from_docker_image {
    cat <<'EOF' | vagrant ssh $1 -- bash -
t=$(mktemp -d skydive.docker.XXX)
pushd $t
sudo docker image save skydive/skydive | tar xf - "*/layer.tar"
find . -name "layer.tar" | xargs -n 1 --replace=XXX sudo tar -C / -xf XXX usr/bin/skydive &>/dev/null || true
popd
rm -rf $t
EOF
}

for mode in $MODES
do
  echo "================== deploying mode $mode ==============================="
  DEPLOYMENT_MODE=$mode vagrant box update
  DEPLOYMENT_MODE=$mode vagrant up --provision-with common ${KEEP_RESOURCES:+--no-destroy-on-error}

  if [ "$mode" = "package" ]; then
      install_skydive_selinux_enforcing analyzer1
      install_skydive_selinux_enforcing agent1
  fi

  vagrant ssh analyzer1 -- sudo ntpdate 10.11.160.238 fr.pool.ntp.org || true
  vagrant ssh agent1 -- sudo ntpdate 10.11.160.238 fr.pool.ntp.org || true

  DEPLOYMENT_MODE=$mode vagrant provision

  vagrant ssh analyzer1 -- sudo cat /etc/skydive/skydive.yml

  if [ "$mode" = "container" ]; then
      install_skydive_from_docker_image analyzer1
      install_skydive_from_docker_image agent1
  fi

  echo "================== external functional test suite ==============================="
  $root/scripts/test.sh -a 192.168.50.10:8082 -e `expr $AGENT_COUNT + $ANALYZER_COUNT` -c -i

  vagrant ssh analyzer1 -- sudo journalctl -n 200 -u skydive-analyzer
  vagrant ssh agent1 -- sudo journalctl -n 200 -u skydive-agent

  vagrant destroy --force
done
