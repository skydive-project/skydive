#!/bin/bash

if [ -n "$(sudo virt-what)" ]; then
    echo "This test must running on baremetal host"
    exit 1
fi

set -e
set -v

dir="$(dirname "$0")"

root=${GOPATH}/src/github.com/skydive-project/skydive
cd $root

export ANALYZER_COUNT=1
export AGENT_COUNT=0
export SKYDIVE_RELEASE=master
export DEPLOYMENT_MODE=package

function vagrant_cleanup {
    set +e
    echo "===== journalctl analyzer1"
    vagrant ssh analyzer1 -c 'sudo journalctl -xe -n 10000 | grep skydive'
    vagrant destroy --force
}
[ "$KEEP_RESOURCES" = "true" ] || trap vagrant_cleanup EXIT

function install_skydive_selinux_enforcing {
    cat <<'EOF' | vagrant ssh $1 -- bash -
sudo setenforce 1
sudo sed -i 's/SELINUX=permissive/SELINUX=enforcing/' /etc/sysconfig/selinux
EOF
}

function run_functional_tests {
  vagrant ssh-config > vagrant-ssh-config
  scp -F vagrant-ssh-config $root/tests/functionals analyzer1:
  rsync -av -e 'ssh -F vagrant-ssh-config' $root/tests/pcaptraces analyzer1:
  ANALYZER1_IP=$(vagrant ssh-config analyzer1 | grep HostName | awk '{print $2}')

  vagrant ssh analyzer1 -c 'for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done'
  vagrant ssh analyzer1 -c 'sudo iptables -F ; sudo iptables -P FORWARD ACCEPT'

  vagrant ssh analyzer1 -c "SKYDIVE_ANALYZERS=\"$ANALYZER1_IP:8082\" sudo -E ./functionals -analyzer.listen $ANALYZER1_IP:8082 -test.v"

  if [ "$DEPLOYMENT_MODE" = "package" ]; then
      echo "===== ausearch AVC on analyzer1 ======"
      vagrant ssh analyzer1 -c 'sudo ausearch -m avc -r' || true
  fi
  rm -f vagrant-ssh-config
}


if [ "$DEVMODE" = "true" ]; then
    make srpm
    VERSION=$(make -s version | cut -d '-' -f 1)
    TAG=$(make -s version | cut -s -d '-' -f 2- | tr '-' '.')
    mock -r epel-7-x86_64 -D "fullver $(make -s version)" --resultdir . --rebuild rpmbuild/SRPMS/skydive-${VERSION}*${TAG}.src.rpm
    mkdir -p rpmbuild/RPMS/{x86_64,noarch}
    mv skydive*-${VERSION}-*${TAG}*.x86_64.rpm rpmbuild/RPMS/x86_64/
    mv skydive*-${VERSION}-*${TAG}*.noarch.rpm rpmbuild/RPMS/noarch/

    make test.functionals.compile GOFLAGS="-race"
fi

cd contrib/vagrant

vagrant box update
vagrant up --provision-with common ${KEEP_RESOURCES:+--no-destroy-on-error}

install_skydive_selinux_enforcing analyzer1
vagrant ssh analyzer1 -- sudo ntpdate 10.11.160.238 fr.pool.ntp.org || true

ANSIBLE_SKIP_TAGS= vagrant provision

# We reboot the kernel so that kernel has user namespaces enabled
vagrant halt
vagrant up

vagrant ssh analyzer1 -- sudo cat /etc/skydive/skydive.yml

timeout 60 bash -c "until vagrant ssh analyzer1 -- curl http://localhost:9200; do sleep 1; done"
timeout 60 bash -c "until vagrant ssh analyzer1 -- curl http://localhost:8082; do sleep 1; done"

run_functional_tests

vagrant ssh analyzer1 -- sudo journalctl -n 200 -u skydive-analyzer

vagrant destroy --force
