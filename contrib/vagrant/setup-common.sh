#!/bin/bash

sudo dnf -y install nfs-utils nfs-utils-lib jq
sudo setenforce 0
sudo sed -i 's/SELINUX=enforcing/SELINUX=disabled/' /etc/sysconfig/selinux
sudo curl -o /usr/bin/skydive -L `curl -s https://api.github.com/repos/skydive-project/skydive/releases/latest | jq --raw-output '.assets[0] | .browser_download_url'`
sudo chmod +x /usr/bin/skydive
