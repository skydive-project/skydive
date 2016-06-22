FROM centos
RUN yum -y install libpcap
RUN VERSION=`curl https://raw.githubusercontent.com/skydive-project/skydive-binaries/nightly-builds/latest`; curl -o /usr/bin/skydive https://media.githubusercontent.com/media/skydive-project/skydive-binaries/nightly-builds/$VERSION/skydive
RUN chmod +x /usr/bin/skydive
COPY start-skydive.sh /usr/bin/start-skydive.sh
RUN mkdir -p /host/run
ENTRYPOINT ["/usr/bin/start-skydive.sh"]
