#!/bin/bash

sudo yum -y install python2 python-yaml libpcap ntpdate audit
sudo setenforce 0
sudo sed -i 's/SELINUX=enforcing/SELINUX=permissive/' /etc/sysconfig/selinux
