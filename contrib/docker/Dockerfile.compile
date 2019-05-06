FROM ubuntu:18.04

ARG UID=1000

VOLUME /root/go/src/github.com/skydive-project/skydive
VOLUME /root/go/.cache/govendor
VOLUME /root/.cache/go-build

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get -y update \
    && apt-get -y install software-properties-common \
    && add-apt-repository -y ppa:ubuntu-toolchain-r/ppa \
    && apt-get -y update \
    && apt-get -y install git make flex bison wget unzip golang libpcap0.8-dev npm \
         clang llvm zlib1g-dev liblzma-dev libc++-dev libc-dev linux-libc-dev libxml2-dev libvirt-dev \
    && rm -rf /var/lib/apt/lists/*

# EBPF requires llvm-6.0 clang-6.0 however the cross compulation docker image can't install them

RUN PROTOC_VER=3.7.1; \
    ARCH=$(uname -m); \
    if [ $ARCH != x86_64 ]; then ARCH=${ARCH%%64*}${ARCH##*64}_64 ; fi ; \
    FILE=protoc-${PROTOC_VER}-linux-${ARCH}.zip; \
    wget --no-verbose https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/${FILE} \
    && unzip ${FILE} \
    && mv bin/protoc /usr/bin/ \
    && mv include/google /usr/include \
    && rm "${FILE}"

RUN mkdir -p /root/go/bin && chmod a+wrx /root/go/bin

RUN arch="$(dpkg --print-architecture)" \
    && wget --no-verbose -O /gosu "https://github.com/tianon/gosu/releases/download/1.11/gosu-${arch##*-}" \
    && chmod a+x /gosu

ENV UID=$UID

RUN chown -R $UID /root/
WORKDIR /root/go/src/github.com/skydive-project/skydive

CMD chown -R $UID /root/go/.cache/govendor \
    && chown -R $UID /root/.cache/go-build \
    && /gosu $UID env \
    HOME=/root \
    GOPATH=/root/go \
    PATH=/root/go/bin:$PATH \
    CGO_ENABLED=1 \
    GOOS=linux \
    make install WITH_OPENCONTRAIL=true WITH_EBPF=true
