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

secret_set() {
        local password=$1
        : ${password:=abc123}
        echo "secret/mysql-pass password: $password"
        kubectl delete secret mysql-pass
        kubectl create secret generic mysql-pass \
                --from-literal="password=$password"
        kubectl label secret mysql-pass app=wordpress
}

secret_get_pod() {
        local tier=$1
        local var=$2
        local pod=$(kubectl get pods -l=app=wordpress,tier=$1 -o jsonpath='{.items[0].metadata.name}')
        local password=$(kubectl exec -it $pod -- sh -c "echo \$$var")
        echo "pod/$pod password: $password"
}

secret_get() {
        secret_get_pod frontend WORDPRESS_DB_PASSWORD
        secret_get_pod mysql MYSQL_ROOT_PASSWORD
}

start() {
        secret
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
        secret-set)
                secret_set $2
                ;;
        secret-get)
                secret_get
                ;;
        url)
                url
                ;;
        *)
                echo "$0 [stop|start|status|secret-get|secret-set|url|help]"
                exit 1
                ;;
esac
exit 0
