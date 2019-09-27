#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

set -e
cd ${GOPATH}/src/github.com/skydive-project/skydive


sudo cat /etc/ssh/sshd_config
sudo perl -i -pe 's|MINIO_.*||g' /etc/ssh/sshd_config

echo ==============
sudo cat /etc/ssh/sshd_config
sudo systemctl restart sshd
