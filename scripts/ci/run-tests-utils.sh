network_setup() {
        sudo iptables -F
        sudo iptables -P FORWARD ACCEPT
        for i in $(find /proc/sys/net/bridge/ -type f); do 
                echo 0 | sudo tee $i
        done
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
        ARGS="$ARGS -graph.output ascii -standalone -analyzer.topology.backend $BACKEND -analyzer.flow.backend $BACKEND"
        export ORIENTDB_ROOT_PASSWORD=root

        if [ "$COVERAGE" != "true" -a "$(uname -m)" != "ppc64le" ]; then
                GOFLAGS="-race"
                export TEST_COVERPROFILE=../functionals-$BACKEND.cover
        fi

        make test.functionals.batch \
                GOFLAGS="$GOFLAGS" VERBOSE=true TAGS="$TAGS" GORACE="history_size=7" TIMEOUT=20m \
                WITH_HELM="$WITH_HELM" WITH_EBPF="$WITH_EBPF" WITH_K8S="$WITH_K8S" WITH_ISTIO="$WITH_ISTIO" \
                ARGS="$ARGS" TEST_PATTERN="$TEST_PATTERN" 2>&1 | tee $LOGFILE
        RETCODE=$?

        go get -f -u github.com/tebeka/go2xunit
        go2xunit -fail -fail-on-race -suite-name-prefix tests \
                -input $LOGFILE -output $TESTFILE
        sed -i 's/\x1b\[[0-9;]*m//g' $TESTFILE

        if [ -e functionals.cover ]; then
                mv functionals.cover $TEST_COVERPROFILE
        fi
}
