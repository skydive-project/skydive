#!/bin/bash

sudo dnf -y install nfs-utils nfs-utils-lib
sudo setenforce 0
sudo sed -i 's/SELINUX=enforcing/SELINUX=disabled/' /etc/sysconfig/selinux
sudo curl -o /usr/bin/skydive -L https://github.com/skydive-project/skydive-binaries/raw/travis-builds/$(curl https://raw.githubusercontent.com/skydive-project/skydive-binaries/travis-builds/latest)/skydive
sudo chmod +x /usr/bin/skydive
