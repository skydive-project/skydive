FROM ubuntu:14.04
RUN apt-get update && apt-get install -y libpcap0.8
COPY bin/skydive /usr/bin/skydive
COPY src/github.com/skydive-project/skydive/contrib/docker/start-skydive.sh /usr/bin/start-skydive.sh
RUN mkdir -p /host/run
ENTRYPOINT ["/usr/bin/start-skydive.sh"]
