#!/bin/bash

PROG=skydive.sh

CONFIG=/etc/skydive/skydive.yml
DRY_RUN=false

OPTIND=1
OPTARG=
OPTION=

while getopts "c:d" OPTION; do
        case $OPTION in
        c)
                CONFIG="$OPTARG"
                ;;
        d)
                DRY_RUN=true
                ;;
        esac
done
shift $((OPTIND-1))

gremlin() {
        local query=$1
        skydive -c $CONFIG client query "$query" | \
                jq '.[] | (.Metadata.K8s.Namespace | if length > 0 then . else "default" end) + "/" + .Metadata.Name' | \
                sed -e s/\"//g
}

reset() {
        local fullname=$1
        local source=$2
        local dest=$3
        if [ -z "$fullname" ]; then
                gremlin "G.V().Has('Manager', 'k8s', 'Type', '$source')"
                exit 1
        fi

        local namespace=${fullname%/*}
        local name=${fullname#*/}
        for i in $(gremlin "G.V().Has('Manager', 'k8s', 'Type', '$source', 'K8s.Namespace', '$namespace', 'K8s.Name', '$name').In().Has('Manager', 'k8s', 'Type', '$dest')"); do
                local namespace=${i%/*}
                local name=${i#*/}
                if $DRY_RUN; then
                        echo kubectl delete -n $namespace $dest $name
                else
                        kubectl delete -n $namespace $dest $name
                fi
        done
}

usage() {
        echo \
"
Usage:
$PROG [flags] [commands]

Available flags:
-c <config>             skydive configuration file ($CONFIG)
-d                      dry run flag ($DRY_RUN)

Available commands:
configmap <name>        post update configmap reset
secret <name>           post update secret reset
"
        exit 1
}

case "$1" in
        configmap)
                reset "$2" configmap pod
                ;;
        secret)
                reset "$2" secret pod
                ;;
        *)
                usage
                ;;
esac
exit 0
