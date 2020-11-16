ARG  BASE=ubuntu:20.04
FROM $BASE
ARG  ARCH=amd64
RUN apt-get -y update \
    && apt-get -y install openvswitch-common libpcap0.8 libxml2 libvirt0 \
    && rm -rf /var/lib/apt/lists/*
COPY skydive.$ARCH /usr/bin/skydive
COPY skydive.yml /etc/skydive.yml
ENTRYPOINT ["/usr/bin/skydive", "--conf", "/etc/skydive.yml"]
