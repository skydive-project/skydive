#!/bin/bash

set -v

if [ -z "$WORKSPACE" ]; then
        echo "need to define WORKSPACE before running script"
        exit 1
fi

go get -f -u github.com/tebeka/go2xunit

DIR="$(dirname "$0")"

BACKEND="memory"
ARGS= \
        " -analyzer.topology.backend $BACKEND" \
        " -analyzer.flow.backend $BACKEND" \
        " -graph.output ascii" \
        " -standalone"
LOGFILE=$WORKSPACE/output.log
TESTFILE=$WORKSPACE/tests.xml

network_setup() {
        sudo iptables -F
        sudo iptables -P FORWARD ACCEPT
        for i in $(find /proc/sys/net/bridge/ -type f); do 
                echo 0 | sudo tee $i
        done
}

helm_setup() {
        . "$DIR/install-minikube.sh" install
        . "$DIR/install-minikube.sh" stop
        . "$DIR/install-minikube.sh" start

        . "$DIR/install-helm.sh" install
        . "$DIR/install-helm.sh" stop
        . "$DIR/install-helm.sh" start
}

helm_teardown() {
        . "$DIR/install-helm.sh" stop
        . "$DIR/install-minikube.sh" stop
}

tests_run() {
        cd ${GOPATH}/src/github.com/skydive-project/skydive

        make test.functionals.batch \
                GOFLAGS="$GOFLAGS" VERBOSE=true TIMEOUT=5m WITH_HELM=true \
                ARGS="$ARGS" TEST_PATTERN=Helm 2>&1 | tee $LOGFILE

        go2xunit -fail -fail-on-race -suite-name-prefix tests \
                -input $LOGFILE -output $TESTFILE
        RETCODE=$?
        sed -i 's/\x1b\[[0-9;]*m//g' $TESTFILE
}

network_setup
helm_setup
. "$DIR/install-go.sh"
tests_run
helm_teardown
exit $RETCODE

# vim: ts=8 sw=8 et
