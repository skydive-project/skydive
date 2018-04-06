#!/bin/bash

sudo yum -y install python2 python-yaml
sudo setenforce 0
sudo sed -i 's/SELINUX=enforcing/SELINUX=disabled/' /etc/sysconfig/selinux
