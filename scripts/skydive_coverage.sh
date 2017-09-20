#!/bin/bash

if [ $1 = "compile" ] ; then
    make govendor genlocalfiles
    go test -v -c -o /tmp/skydive_coverage $COVERFLAGS github.com/skydive-project/skydive/tests/coverage
    exit $?
fi

config=""
command=""
while test ${#} -gt 0
do
    if [ $1 = "-c" ]; then
        config=$2
    fi
    if [ $1 = "agent" -o $1 = "analyzer" ]; then
        command=$1
    fi
    shift
done

/tmp/skydive_coverage -test.coverprofile=$COVERFILE -test.v -config=$config $command &
pid=$(pgrep -f "/tmp/skydive_coverage -test.coverprofile=$COVERFILE -test.v -config=$config $command")
if [ -n "$PIDFILE" ] ; then
    echo $pid > $PIDFILE
fi
wait $pid
