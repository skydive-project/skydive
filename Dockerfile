FROM ubuntu:20.04 as builder
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get -y update \
    && apt-get -y install build-essential git-core golang npm openvswitch-common libpcap0.8 libpcap0.8-dev libxml2-dev protobuf-compiler libprotobuf-dev libvirt-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /go/src/github.com/skydive-project/skydive
COPY . .
ARG GOPATH=/go
RUN make build

FROM ubuntu:20.04 as skydive
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get -y update \
    && apt-get -y install golang npm openvswitch-common libpcap0.8 libvirt0 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /go/src/github.com/skydive-project/skydive/skydive /usr/bin/skydive
COPY contrib/docker/skydive.yml /etc/skydive.yml
ENTRYPOINT ["/usr/bin/skydive", "--conf", "/etc/skydive.yml"]
