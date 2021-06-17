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

export ANALYZER_COUNT=2
export AGENT_COUNT=1
export SKYDIVE_RELEASE=master

function vagrant_cleanup {
    set +e
    echo "===== journalctl agent1"
    vagrant ssh agent1 -c 'sudo journalctl -xe | grep skydive'
    echo "===== journalctl analyzer1"
    vagrant ssh analyzer1 -c 'sudo journalctl -xe | grep skydive'
    echo "===== journalctl analyzer2"
    vagrant ssh analyzer2 -c 'sudo journalctl -xe | grep skydive'
    vagrant destroy --force
}
[ "$KEEP_RESOURCES" = "true" ] || trap vagrant_cleanup EXIT

function run_functional_tests {
  vagrant ssh-config > vagrant-ssh-config
  scp -F vagrant-ssh-config $root/tests/functionals agent1:
  rsync -av -e 'ssh -F vagrant-ssh-config' $root/tests/pcaptraces agent1:
  AGENT1_IP=$(vagrant ssh-config agent1 | grep HostName | awk '{print $2}')
  ANALYZER1_IP=$(vagrant ssh-config analyzer1 | grep HostName | awk '{print $2}')
  ANALYZER2_IP=$(vagrant ssh-config analyzer2 | grep HostName | awk '{print $2}')

  vagrant ssh agent1 -c 'for i in $(find /proc/sys/net/bridge/ -type f) ; do echo 0 | sudo tee $i ; done'
  vagrant ssh agent1 -c 'sudo iptables -F ; sudo iptables -P FORWARD ACCEPT'

  if [ "$mode" = "container" ]; then
      OPT="-nooftests"
  fi

  vagrant ssh agent1 -c "AGENT1_IP=$AGENT1_IP SKYDIVE_ANALYZERS=\"$ANALYZER1_IP:8082 $ANALYZER2_IP:8082\" sudo -E ./functionals -analyzer.listen 0.0.0.0:8082 -agenttestsonly -test.v $OPT"

  if [ "$mode" = "package" ]; then
      for a in analyzer1 analyzer2 agent1; do
          echo "===== ausearch AVC on $a ======"
          vagrant ssh $a -c 'sudo ausearch -m avc -r' || true
      done
  fi
  rm -f vagrant-ssh-config
}

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
      install_skydive_selinux_enforcing analyzer2
      install_skydive_selinux_enforcing agent1
  fi

  for a in analyzer1 analyzer2 agent1; do
      echo "$a"
      vagrant ssh $a -- sudo ntpdate 10.11.160.238 fr.pool.ntp.org || true
  done

  DEPLOYMENT_MODE=$mode vagrant provision

  for a in analyzer1 analyzer2 agent1; do
      echo "$a"
      vagrant ssh $a -- sudo cat /etc/skydive/skydive.yml
  done

  vagrant ssh analyzer1 -- sudo journalctl -n 100 -u skydive-analyzer
  vagrant ssh analyzer2 -- sudo journalctl -n 100 -u skydive-analyzer
  vagrant ssh agent1 -- sudo journalctl -n 100 -u skydive-agent

  for a in analyzer1 analyzer2; do
      echo "api/status $a"
      vagrant ssh $a -- curl http://localhost:8082/api/status
  done
  echo "api/status agent1"
  out=$(mktemp)
  n=0
  count=10
  while [ "$n" -ne $ANALYZER_COUNT ]; do
      vagrant ssh agent1 -- curl http://localhost:8081/api/status | tee "$out"
      n=$(cat "$out" | jq ".Analyzers | length")
      count=$[count-1]
      if [ $count -eq 0 ]; then
          exit 1
      fi
      sleep 0.5
  done
  rm -f "$out"

  if [ "$mode" = "container" ]; then
      for a in analyzer1 analyzer2 agent1; do
          echo "$a"
          install_skydive_from_docker_image $a
      done
  fi

  echo "================== external functional test suite ==============================="
  $root/scripts/test.sh -a 192.168.50.10:8082 -e `expr $AGENT_COUNT + $ANALYZER_COUNT` -c -i

  vagrant ssh analyzer1 -- sudo journalctl -n 200 -u skydive-analyzer
  vagrant ssh analyzer2 -- sudo journalctl -n 200 -u skydive-analyzer
  vagrant ssh agent1 -- sudo journalctl -n 200 -u skydive-agent

  if [ "$mode" = "package" ]; then
      for a in analyzer1 analyzer2 agent1; do
          echo "$a"
          install_skydive_selinux_enforcing $a
      done
  fi

  echo "================== gremlin test ==============================="
  for a in analyzer1 analyzer2; do
      echo "$a"
      vagrant ssh $a -c 'set -e; skydive client query "g.V()"'
  done

  if [ "$mode" != "container" ]; then
      sleep 10
      echo "================== functional test suite ==============================="
      run_functional_tests
  fi

  vagrant ssh analyzer1 -- sudo journalctl -n 200 -u skydive-analyzer
  vagrant ssh analyzer2 -- sudo journalctl -n 200 -u skydive-analyzer
  vagrant ssh agent1 -- sudo journalctl -n 200 -u skydive-agent

  vagrant destroy --force
done
