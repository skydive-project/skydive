ARG BASE=gcr.io/distroless/base
FROM $BASE
COPY skydive /usr/bin/skydive
COPY skydive.yml /etc/skydive.yml
ENTRYPOINT ["/usr/bin/skydive", "--conf", "/etc/skydive.yml"]
