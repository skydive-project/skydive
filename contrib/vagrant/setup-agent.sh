#!/bin/sh

sudo dnf -y install openvswitch docker
sudo systemctl enable openvswitch.service
sudo systemctl enable docker.service
sudo systemctl start openvswitch.service
sudo systemctl start docker.service

sudo mkdir -p /etc/skydive
agent=`hostname | tr -d "a-z"`
sudo tee /etc/skydive/skydive.yml << EOF
agent:
  analyzers: 192.168.50.10:8082
  flow:
    probes:
      - ovssflow
      - gopacket
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - fabric
    fabric:
      - TOR1[Name=tor1] -> TOR1_PORT${agent}[Name=port${agent}, MTU=1500]
      - TOR1_PORT${agent} -> local/eth1
etcd:
  client_timeout: 100
EOF
sudo curl -o /usr/lib/systemd/system/skydive-agent.service https://raw.githubusercontent.com/skydive-project/skydive/master/contrib/systemd/skydive-agent.service
sudo systemctl daemon-reload
sudo systemctl enable skydive-agent.service
