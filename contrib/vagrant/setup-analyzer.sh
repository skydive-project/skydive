#!/bin/sh

sudo rpm --import https://packages.elastic.co/GPG-KEY-elasticsearch
cat > /etc/yum.repos.d/elasticsearch.repo <<EOF
[elasticsearch-2.x]
name=Elasticsearch repository for 2.x packages
baseurl=https://packages.elastic.co/elasticsearch/2.x/centos
gpgcheck=1
gpgkey=https://packages.elastic.co/GPG-KEY-elasticsearch
enabled=1
EOF
sudo dnf -y install elasticsearch java-1.8.0-openjdk
sudo bash -c "echo 'network.host: 0.0.0.0' >> /etc/elasticsearch/elasticsearch.yml"
sudo systemctl enable elasticsearch.service
sudo systemctl start elasticsearch.service
sudo mkdir -p /etc/skydive
sudo tee /etc/skydive/skydive.yml << EOF
analyzer:
  listen: 0.0.0.0:8082
  flowtable_expire: 60
  flowtable_update: 5
  flowtable_agent_ratio: 0.5
  storage: elasticsearch
  topology:
    fabric:
      - TOR1[Name=tor1] -> TOR1_PORT1[Name=port1, MTU=1500]
      - TOR1[Name=tor1] -> TOR1_PORT2[Name=port2, MTU=1500]
      - TOR1_PORT1 -> *[Type=host,Name=agent1]/eth1
      - TOR1_PORT2 -> *[Type=host,Name=agent2]/eth1
etcd:
  client_timeout: 100
graph:
  backend: elasticsearch
elasticsearch:
  addr: 127.0.0.1:9200
EOF
sudo curl -o /usr/lib/systemd/system/skydive-analyzer.service https://raw.githubusercontent.com/skydive-project/skydive/master/contrib/systemd/skydive-analyzer.service
sudo systemctl daemon-reload
sudo systemctl enable skydive-analyzer.service
