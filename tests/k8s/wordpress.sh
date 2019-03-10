#!/bin/bash

SRC=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

WORDPRESS=https://k8s.io/examples/application/wordpress

apply() {
        kubectl apply $*
}

delete() {
        kubectl delete --grace-period=0 --force $*
}

stop() {
        kubectl delete secret mysql-pass
        delete deployment -l app=wordpress
        delete service -l app=wordpress
        delete pvc -l app=wordpress
}

start() {
        local password=abc123
        kubectl create secret generic mysql-pass \
                --from-literal="password=$password"
        apply -f $SRC/mysql-deployment.yaml
        apply -f $SRC/wordpress-deployment.yaml
}

status() {
        kubectl get service -l app=wordpress
        kubectl get deployment -l app=wordpress
        kubectl get pods -l app=wordpress
        kubectl get pvc -l app=wordpress
}

url() {
        minikube service wordpress --url
}

case "$1" in
        stop)
                stop
                ;;
        start)
                start
                ;;
        status)
                status
                ;;
        url)
                url
                ;;
        *)
                echo "$0 [stop|start|status|url|help]"
                exit 1
                ;;
esac
exit 0
