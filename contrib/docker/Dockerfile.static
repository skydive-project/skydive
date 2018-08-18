ARG  BASE=ubuntu:18.04
FROM $BASE
ARG  ARCH=x86_64
COPY skydive.$ARCH /usr/bin/skydive
COPY skydive.yml /etc/skydive.yml
ENTRYPOINT ["/usr/bin/skydive", "--conf", "/etc/skydive.yml"]
