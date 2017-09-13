#!/bin/bash

sudo dnf -y install python2
sudo setenforce 0
sudo sed -i 's/SELINUX=enforcing/SELINUX=disabled/' /etc/sysconfig/selinux
