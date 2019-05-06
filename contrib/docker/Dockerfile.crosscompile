FROM ubuntu:18.04

ARG TARGET_ARCH=s390x
ARG TARGET_GOARCH=$TARGET_ARCH
ARG DEBARCH=$TARGET_GOARCH
ARG UID=1000

VOLUME /root/go/src/github.com/skydive-project/skydive
VOLUME /root/go/.cache/govendor

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get -y update \
    && apt-get -y install software-properties-common \
    && add-apt-repository -y ppa:ubuntu-toolchain-r/ppa \
    && dpkg --add-architecture $DEBARCH \
    && echo "deb [arch=$DEBARCH] http://ports.ubuntu.com/ubuntu-ports bionic main universe" >> /etc/apt/sources.list \
    && echo "deb [arch=$DEBARCH] http://ports.ubuntu.com/ubuntu-ports bionic-updates main universe" >> /etc/apt/sources.list \
    && apt-get -y update; \
    apt-get -y install git make flex bison wget unzip golang npm \
                   zlib1g-dev:$DEBARCH \
                   liblzma-dev:$DEBARCH \
                   libc++-dev:$DEBARCH \
                   libc-dev:$DEBARCH \
                   libpcap0.8-dev:$DEBARCH \
                   linux-libc-dev:$DEBARCH \
                   gcc-${TARGET_ARCH}-linux-gnu \
    && rm -rf /var/lib/apt/lists/*

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

ENV TARGET_ARCH=$TARGET_ARCH
ENV TARGET_GOARCH=$TARGET_GOARCH

CMD chown -R $UID /root/go/.cache/govendor \
    && /gosu $UID env \
    HOME=/root \
    GOPATH=/root/go \
    PATH=/root/go/bin:$PATH \
    CGO_ENABLED=1 \
    CC=${TARGET_ARCH}-linux-gnu-gcc \
    GOOS=linux \
    GOARCH=$TARGET_GOARCH \
    make govendor compile.static WITH_OPENCONTRAIL=false WITH_LIBVIRT=false WITH_EBPF=true
