#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go-deps.sh"

# merge all together
echo "mode: atomic" > cover.out
grep -h -v "^mode:" *.cover | grep -v "skydive/statics" | awk '{ stmt[$1] += $2; count[$1] += $3 } END{ for(e in stmt) { print e, stmt[e], count[e] } }' >> cover.out

go get github.com/t-yuki/gocover-cobertura
go tool cover -func=cover.out
gocover-cobertura < cover.out > cover.out.xml
