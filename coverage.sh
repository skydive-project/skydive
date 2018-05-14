#!/bin/bash
# Generate test coverage statistics for Go packages.
#
# Works around the fact that `go test -coverprofile` currently does not work
# with multiple packages, see https://code.google.com/p/go/issues/detail?id=6909
#
# Usage: script/coverage [--html|--coveralls]
#
#     --html      Additionally create HTML report and open it in browser
#     --coveralls Push coverage statistics to coveralls.io
#

set -e

workdir=".cover"
profile="$workdir/cover.out"
mode=count
units=1
functionals=1
scale=1
coveralls=0
backend=elasticsearch

generate_cover_data() {
    rm -rf "$workdir"
    mkdir "$workdir"

    if [ "$units" -eq 1 ]; then
        # unit test
        make test COVERAGE=true COVERAGE_WD=$workdir COVERAGE_MODE=$mode
    fi

    PKG=$(go list ./... | grep -v -e '/tests' -e '/vendor' | tr '\n' ',' | sed -e 's/,$//')
    if [ "$functionals" -eq 1 ]; then
        # add fonctional testing
        export SKYDIVE_ANALYZERS=localhost:8082

        coverfile="../$workdir/functional.cover"
        make test.functionals.batch VERBOSE=true WITH_EBPF=true WITH_K8S=false TIMEOUT=20m GOFLAGS="-cover -covermode=$mode -coverpkg=$PKG" ARGS="$ARGS -test.coverprofile=$coverfile -standalone -analyzer.topology.backend $backend -analyzer.flow.backend $backend" TEST_PATTERN=$TEST_PATTERN
    fi

    if [ "$scale" -eq 1 ]; then
        # scale test
        export SKYDIVE_ANALYZERS=localhost:8082
        export ELASTICSEARCH=localhost:9200
        export TLS=true
        coverfile="../$workdir/scale.cover"
        export SKYDIVE="${GOPATH}/src/github.com/skydive-project/skydive/scripts/skydive_coverage.sh"
        export COVERFLAGS=" -cover -covermode=$mode -coverpkg=$PKG "
        export FLOW_PROTOCOL=websocket
        export SKYDIVE_LOGGING_LEVEL=DEBUG

        curl -XDELETE 'localhost:9200/skydive*'
        $SKYDIVE compile

        make test.functionals WITH_SCALE=true VERBOSE=true TIMEOUT=10m TEST_PATTERN=Scale
        cp /tmp/skydive-scale/*.cover "$workdir"/
    fi

    # merge all together
    echo "mode: $mode" > "$profile"
    grep -h -v "^mode:" "$workdir"/*.cover | grep -v "skydive/statics" | awk '{ stmt[$1] += $2; count[$1] += $3 } END{ for(e in stmt) { print e, stmt[e], count[e] } }' >> "$profile"
}

show_cover_report() {
    if [ "$1" == "xml" ]; then
        go get github.com/t-yuki/gocover-cobertura
        gocover-cobertura < $profile > $profile.xml
    else
        go tool cover -${1}="$profile"
    fi
}

push_to_coveralls() {
    echo "Pushing coverage statistics to coveralls.io"
    goveralls -coverprofile="$profile"
}

push_to_codecov() {
    if [ -n $CODECOV_TOKEN ]; then
        echo "Pushing coverage statistics to codecov.io"
        cp "$profile" coverage.txt
        bash <(curl -s https://codecov.io/bash) -t $CODECOV_TOKEN
    fi
}

format=func
for arg in "$@"
do
    case "$arg" in
    "")
        ;;
    --html)
        format=html ;;
    --xml)
        format=xml ;;
    --no-units)
        units=0 ;;
    --no-functionals)
        functionals=0 ;;
    --no-scale)
        scale=0 ;;
    --orientdb)
        backend=orientdb ;;
    --coveralls)
        coveralls=1 ;;
    *)
        echo >&2 "error: invalid option: $1"; exit 1 ;;
    esac
done

generate_cover_data
show_cover_report $format
[ "$coveralls" -eq 1 ] && push_to_coveralls
push_to_codecov
