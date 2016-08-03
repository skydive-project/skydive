FROM centos
RUN yum -y install libpcap
COPY bin/skydive /usr/bin/skydive
COPY src/github.com/skydive-project/skydive/contrib/docker/start-skydive.sh /usr/bin/start-skydive.sh
RUN mkdir -p /host/run
ENTRYPOINT ["/usr/bin/start-skydive.sh"]
