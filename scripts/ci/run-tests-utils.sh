#!/bin/bash

set -v

network_setup() {
        sudo iptables -F
        sudo iptables -P FORWARD ACCEPT
        for i in $(find /proc/sys/net/bridge/ -type f); do 
                echo 0 | sudo tee $i
        done
}

mem_prof() {
        echo start memory profiling
        while(true); do
                echo trigger memory profiling snapshot
                sudo pkill -USR2 functionals 2>/dev/null || true
                sleep 10
                sudo mv /tmp/skydive-memory.prof /tmp/skydive-memory.prof.$( date +%T ) 2>/dev/null || true
        done
}

es_setup() {
    docker run -d --name elasticsearch  -p 9201:9200 -p 9301:9300 -e "discovery.type=single-node" elasticsearch:7.7.1
}

es_cleanup() {
    docker stop elasticsearch
    docker rm elasticsearch
}

tests_run() {
        cd ${GOPATH}/src/github.com/skydive-project/skydive

        if [ -z "$WORKSPACE" ]; then
                echo "need to define WORKSPACE before running script"
                exit 1
        fi
        LOGFILE=$WORKSPACE/output.log
        TESTFILE=$WORKSPACE/tests.xml

        BACKEND=${BACKEND:-memory}

        if [ "$BACKEND" = "elasticsearch" ]; then
                es_setup && trap es_cleanup EXIT
        fi

        ARGS="$ARGS -standalone -analyzer.topology.backend $BACKEND -analyzer.flow.backend $BACKEND"
        export ORIENTDB_ROOT_PASSWORD=root

        if [ "$COVERAGE" != "true" -a "$(uname -m)" != "ppc64le" ]; then
                export TEST_COVERPROFILE=../functionals-$BACKEND.cover
        fi

        if [ "$WITH_PROF" = "true" ]; then
                mem_prof&
                MEMPROFPID=$!
        fi

        make test.functionals.batch \
                GOFLAGS="$GOFLAGS" VERBOSE=true TAGS="$TAGS" GORACE="history_size=7" TIMEOUT=20m \
                WITH_HELM="$WITH_HELM" WITH_EBPF="$WITH_EBPF" WITH_EBPF_DOCKER_BUILDER=true \
                WITH_K8S="$WITH_K8S" WITH_ISTIO="$WITH_ISTIO" \
                WITH_PROF="$WITH_PROF" WITH_OVNK8S="$WITH_OVNK8S" ARGS="$ARGS" TEST_PATTERN="$TEST_PATTERN" 2>&1 | tee $LOGFILE
        RETCODE=$?

        if [ "$WITH_PROF" = "true" ]; then
                kill $MEMPROFPID 2>/dev/null
        fi

        go run github.com/tebeka/go2xunit -fail -fail-on-race -suite-name-prefix tests \
                -input $LOGFILE -output $TESTFILE
        sed -i 's/\x1b\[[0-9;]*m//g' $TESTFILE

        if [ -e functionals.cover ]; then
                mv functionals.cover $TEST_COVERPROFILE
        fi
}
