#!/bin/sh

sudo dnf -y install openvswitch docker
sudo systemctl enable openvswitch.service
sudo systemctl enable docker.service
sudo systemctl start openvswitch.service
sudo systemctl start docker.service

sudo mkdir -p /etc/skydive
sudo tee /etc/skydive/skydive.yml << EOF
analyzers:
  - 192.168.50.10:8082
agent:
  topology:
    probes:
      - ovsdb
      - docker
etcd:
  client_timeout: 100
EOF
sudo curl -o /usr/lib/systemd/system/skydive-agent.service https://raw.githubusercontent.com/skydive-project/skydive/master/contrib/systemd/skydive-agent.service
sudo systemctl daemon-reload
sudo systemctl enable skydive-agent.service
