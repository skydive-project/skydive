#!/bin/bash

set -v

sudo dnf -y install gcc glibc-static xz-static zlib-static flex bison byacc libxml2-static

git clone https://github.com/the-tcpdump-group/libpcap.git
cd libpcap
git checkout libpcap-1.5.3
./configure --prefix=/usr/local --disable-shared --disable-dbus --disable-bluetooth --disable-canusb
make
sudo make install
cd ..
