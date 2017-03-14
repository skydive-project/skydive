#!/bin/sh
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

generate_cover_data() {
    rm -rf "$workdir"
    mkdir "$workdir"

    # unit test
    PKG=$(go list ./... | grep -v -e '/tests' -e '/vendor')
    for pkg in ${PKG}; do
        coverfile="$workdir/$(echo $pkg | tr / -).cover"
        govendor test -tags "${TAGS} test" -timeout 6m -covermode="$mode" -coverprofile="$coverfile" "$pkg"
    done

    # add fonctional testing
    export SKYDIVE_ANALYZERS=localhost:8082

    coverfile="../$workdir/functional.cover"
    PKG=$(go list ./... | grep -v -e '/tests' -e '/vendor' | tr '\n' ',' | sed -e 's/,$//')
    make test.functionals.batch VERBOSE=true TIMEOUT=20m GOFLAGS="-cover -covermode=$mode -coverpkg=$PKG" ARGS="-test.coverprofile=$coverfile -standalone -graph.backend elasticsearch -storage.backend elasticsearch"

    # merge all together
    echo "mode: $mode" > "$profile"
    grep -h -v "^mode:" "$workdir"/*.cover | grep -v "skydive/statics" | awk '{ stmt[$1] += $2; count[$1] += $3 } END{ for(e in stmt) { print e, stmt[e], count[e] } }' >> "$profile"
}

show_cover_report() {
    go tool cover -${1}="$profile"
}

push_to_coveralls() {
    echo "Pushing coverage statistics to coveralls.io"
    goveralls -coverprofile="$profile"
}

generate_cover_data
show_cover_report func
case "$1" in
"")
    ;;
--html)
    show_cover_report html ;;
--coveralls)
    push_to_coveralls ;;
*)
    echo >&2 "error: invalid option: $1"; exit 1 ;;
esac
