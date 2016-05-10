FROM centos
RUN yum -y install libpcap
RUN VERSION=`curl https://raw.githubusercontent.com/redhat-cip/skydive/nightly-builds/latest`; curl -o /usr/bin/skydive https://media.githubusercontent.com/media/redhat-cip/skydive/nightly-builds/$VERSION/skydive
RUN chmod +x /usr/bin/skydive
COPY start-skydive.sh /usr/bin/start-skydive.sh
ENTRYPOINT ["/usr/bin/start-skydive.sh"]
