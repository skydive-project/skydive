#!/bin/bash

if [ $# -lt 2 ]; then
    echo "usage: $0 [arch] [goarch] [debarch]"
    exit 1
fi

TARGET_ARCH=$1
TARGET_GOARCH=$2
DEBARCH=$3

cd

export DEBIAN_FRONTEND=noninteractive

apt -y update
DEBIAN_FRONTEND=noninteractive apt -y install software-properties-common
add-apt-repository -y ppa:ubuntu-toolchain-r/ppa
apt -y update
apt -y install gcc-${TARGET_ARCH}-linux-gnu git make flex bison wget unzip golang libpcap0.8-dev

git clone https://github.com/the-tcpdump-group/libpcap.git
cd libpcap/
CC=${TARGET_ARCH}-linux-gnu-gcc ./configure --prefix=/usr/local --disable-shared --disable-dbus --disable-bluetooth --host=${TARGET_ARCH}-linux-gnu
make
make install
cd ..

wget https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip
unzip protoc-3.1.0-linux-x86_64.zip
mv bin/protoc /usr/bin/

# Install package for static compilation
dpkg --add-architecture ppc64el
echo "deb [arch=ppc64el] http://ports.ubuntu.com/ubuntu-ports bionic main universe" >> /etc/apt/sources.list
apt -y update
apt -y install zlib1g-dev:$DEBARCH \
               liblzma-dev:$DEBARCH \
               libc++-dev:$DEBARCH \
               libc-dev:$DEBARCH \
               libpcap0.8-dev:$DEBARCH \
               linux-libc-dev:$DEBARCH

export GOPATH=/root/go
cd go/src/github.com/skydive-project/skydive
export PATH=$GOPATH/bin:$PATH
export CGO_ENABLED=1
export CC=${TARGET_ARCH}-linux-gnu-gcc
export GOOS=linux
export GOARCH=$TARGET_GOARCH
make govendor compile.static WITH_OPENCONTRAIL=false WITH_EBPF=true
rm -rf vendor/github.com/weaveworks/tcptracer-bpf

cp $GOPATH/bin/${GOOS}_${GOARCH}/skydive contrib/docker/