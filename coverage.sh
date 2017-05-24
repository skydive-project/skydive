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
functionals=1
coveralls=0

generate_cover_data() {
    rm -rf "$workdir"
    mkdir "$workdir"

    # unit test
    [ -z "$PKG" ] && PKG=$(go list ./... | grep -v -e '/tests' -e '/vendor')
    for pkg in ${PKG}; do
        coverfile="$workdir/$(echo $pkg | tr / -).cover"
        make test GOFLAGS="-covermode=$mode -coverprofile=$coverfile" UT_PACKAGES=$pkg
    done

    if [ "$functionals" -eq 1 ];
    then
        # add fonctional testing
        export SKYDIVE_ANALYZERS=localhost:8082

        coverfile="../$workdir/functional.cover"
        PKG=$(go list ./... | grep -v -e '/tests' -e '/vendor' | tr '\n' ',' | sed -e 's/,$//')
        make test.functionals.batch VERBOSE=true TIMEOUT=20m GOFLAGS="-cover -covermode=$mode -coverpkg=$PKG" ARGS="$ARGS -test.coverprofile=$coverfile -standalone -graph.backend elasticsearch -storage.backend elasticsearch" TEST_PATTERN=$TEST_PATTERN
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
    --no-functionals)
        functionals=0 ;;
    --coveralls)
        coveralls=1 ;;
    *)
        echo >&2 "error: invalid option: $1"; exit 1 ;;
    esac
done

generate_cover_data
show_cover_report $format
[ "$coveralls" -eq 1 ] && push_to_coveralls
