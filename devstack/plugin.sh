#!/bin/bash

#    Licensed under the Apache License, Version 2.0 (the "License"); you may
#    not use this file except in compliance with the License. You may obtain
#    a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
#    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
#    License for the specific language governing permissions and limitations
#    under the License.

# The path where go binaries have to be installed
GOROOT=${GOROOT:-/opt/go}

# golang version. Skydive needs at least version 1.9
GO_VERSION=${GO_VERSION:-1.9}

# GOPATH where the go src, pkgs are installed
GOPATH=/opt/stack/go

# Keystone API version used by Skydive
SKYDIVE_KEYSTONE_API_VERSION=${SKYDIVE_KEYSTONE_API_VERSION:-v3}

# Address on which skydive analyzer process listens for connections.
# Must be in ip:port format
SKYDIVE_ANALYZER_LISTEN=${SKYDIVE_ANALYZER_LISTEN:-$SERVICE_HOST:8082}

# Inform the agent about the address on which analyzers are listening
SKYDIVE_ANALYZERS=${SKYDIVE_ANALYZERS:-$SKYDIVE_ANALYZER_LISTEN}

# Configure the skydive analyzer with the etcd server address
SKYDIVE_ANALYZER_ETCD=${SKYDIVE_ANALYZER_ETCD:-$SERVICE_HOST:12379}

# ip:port address on which skydive agent listens for connections.
SKYDIVE_AGENT_LISTEN=${SKYDIVE_AGENT_LISTEN:-"127.0.0.1:8081"}

# The path for the generated skydive configuration file
SKYDIVE_CONFIG_FILE=${SKYDIVE_CONFIG_FILE:-"/etc/skydive/skydive.yaml"}

# List of agent probes to be used by the agent
SKYDIVE_AGENT_PROBES=${SKYDIVE_AGENT_PROBES:-"ovsdb neutron"}

# List of analyzer probes to be used by the analyzer
SKYDIVE_ANALYZER_PROBES=${SKYDIVE_ANALYZER_PROBES:-""}

# Remote port for ovsdb server.
SKYDIVE_OVSDB_REMOTE_PORT=${SKYDIVE_OVSDB_REMOTE_PORT:-}

# Default log level
SKYDIVE_LOGLEVEL=${SKYDIVE_LOGLEVEL:-INFO}

# Storage used by the analyzer to store flows
SKYDIVE_FLOWS_STORAGE=${SKYDIVE_FLOWS_STORAGE:-"memory"}

# Storage used by the analyzer to store the graph
SKYDIVE_GRAPH_STORAGE=${SKYDIVE_GRAPH_STORAGE:-"memory"}

# Installation mode: source, binary, release
SKYDIVE_INSTALL_MODE=${SKYDIVE_INSTALL_MODE:-"source"}

# List of public interfaces for the agents to register in fabric
# ex: "devstack1/eth0 devstack2/eth1"
if [ "x$PUBLIC_INTERFACE" != "x" ]; then
    SKYDIVE_PUBLIC_INTERFACES=${SKYDIVE_PUBLIC_INTERFACES:-$LOCAL_HOSTNAME/$PUBLIC_INTERFACE}
fi

ELASTICSEARCH_BASE_URL=https://download.elastic.co/elasticsearch/release/org/elasticsearch/distribution
ELASTICSEARCH_VERSION=5.6.14

USE_ELASTICSEARCH=0
if [ "${SKYDIVE_FLOWS_STORAGE}" == "elasticsearch" ] || [ "${SKYDIVE_GRAPH_STORAGE}" == "elasticsearch" ]; then
    USE_ELASTICSEARCH=1
fi


function install_protoc {
    mkdir -p $DEST/protoc
    pushd $DEST/protoc
    wget https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip
    unzip -o -u protoc-3.1.0-linux-x86_64.zip
    popd
    export PATH=$DEST/protoc/bin:${PATH}
}

function install_go {
    if [[ `uname -m` == *"64" ]]; then
        arch=amd64
    else
        arch=386
    fi

    if [ ! -d $GOROOT ]; then
        sudo mkdir -p $GOROOT
        safe_chown -R $STACK_USER $GOROOT
        safe_chmod 0755 $GOROOT
        curl -s -L https://storage.googleapis.com/golang/go$GO_VERSION.linux-$arch.tar.gz | tar -C `dirname $GOROOT` -xzf -
    fi
    export GOROOT=$GOROOT
    export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
    export GOPATH=$GOPATH
}

function download_elasticsearch {
    if [ ! -f ${TOP_DIR}/files/elasticsearch-${ELASTICSEARCH_VERSION}.* ]; then
        if is_ubuntu; then
            wget https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-${ELASTICSEARCH_VERSION}.deb \
                -O ${TOP_DIR}/files/elasticsearch-${ELASTICSEARCH_VERSION}.deb
        elif is_fedora; then
            wget https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-${ELASTICSEARCH_VERSION}.rpm \
                -O ${TOP_DIR}/files/elasticsearch-${ELASTICSEARCH_VERSION}.noarch.rpm
        fi
    fi
}

function pre_install_skydive {
    install_protoc
    install_go
    if [ ${USE_ELASTICSEARCH} -eq 1 ]; then
        if is_service_enabled skydive-analyzer; then
            download_elasticsearch
            export ELASTICSEARCH_VERSION
            $TOP_DIR/pkg/elasticsearch.sh install
        fi
    fi
}

function install_from_source {
    if is_fedora ; then
        install_package libpcap-devel npm libvirt-devel
    else
        install_package libpcap-dev npm libvirt-dev
    fi
    SKYDIVE_SRC=$GOPATH/src/github.com/skydive-project
    mkdir -p $SKYDIVE_SRC
    if [ ! -d $SKYDIVE_SRC/skydive ]; then
        mv $DEST/skydive $SKYDIVE_SRC/
        ln -s $SKYDIVE_SRC/skydive $DEST/skydive
    fi
    cd $SKYDIVE_SRC/skydive
    make install

    sudo ln -s $GOPATH/bin/skydive /usr/bin/skydive
}

function install_latest_binary {
    curl -Lo /tmp/skydive https://github.com/skydive-project/skydive-binaries/raw/jenkins-builds/skydive-latest
    chmod +x /tmp/skydive
    sudo mv /tmp/skydive /usr/bin/skydive
}

function install_latest_release {
    local version=$( curl -s https://api.github.com/repos/skydive-project/skydive/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")' )
    curl -Lo /tmp/skydive https://github.com/skydive-project/skydive/releases/download/${version}/skydive
    chmod +x /tmp/skydive
    sudo mv /tmp/skydive /usr/bin/skydive
}

function install_skydive {
    if [ ! -f $GOPATH/bin/skydive ]; then
        case $SKYDIVE_INSTALL_MODE in

            source) install_from_source;;
            binary) install_latest_binary;;
            release) install_latest_release;;

        esac

    	sudo chown root:root /usr/bin/skydive
    fi
}

function join {
    local d=$1; shift; echo -n "$1"; shift; printf "%s\n" "${@/#/$d}";
}

function get_probes_for_config {
    printf "%s" "$(join '      - ' '' $*)";
}

function get_fabric_config {
    for hostintf in $SKYDIVE_PUBLIC_INTERFACES; do
        host=${hostintf%/*}
        intf=${hostintf#*/}
        echo "      - TOR[Name=TOR Switch, Type=switch] -> PORT_${host}[Name=${host} Port, Type=port]"
        echo "      - PORT_${host} --> *[Type=host, Name=${host}]/${intf}"
    done
}

function configure_skydive {
    sudo mkdir -p `dirname $SKYDIVE_CONFIG_FILE`
    cat <<EOF | sudo tee $SKYDIVE_CONFIG_FILE
auth:
  type: keystone
  analyzer_username: admin
  analyzer_password: $ADMIN_PASSWORD
  keystone:
    auth_url: ${KEYSTONE_AUTH_URI}/${SKYDIVE_KEYSTONE_API_VERSION}
    tenant_name: admin
    domain_name: $SERVICE_DOMAIN_NAME

logging:
  level: $SKYDIVE_LOGLEVEL

etcd:
  servers:
    - http://$SKYDIVE_ANALYZER_ETCD
  data_dir: /tmp/skydive-etcd
  listen: $SKYDIVE_ANALYZER_ETCD

analyzers:
  - $SKYDIVE_ANALYZERS

agent:
  listen: $SKYDIVE_AGENT_LISTEN
  topology:
    probes:
$(get_probes_for_config $SKYDIVE_AGENT_PROBES)

    neutron:
      auth_url: ${KEYSTONE_AUTH_URI}/${SKYDIVE_KEYSTONE_API_VERSION}
      username: admin
      password: $ADMIN_PASSWORD
      tenant_name: admin
      region_name: RegionOne
      domain_name: $SERVICE_DOMAIN_NAME

ovs:
  oflow:
    enable: true
    native: true

analyzer:
EOF

    if [ "$SKYDIVE_FLOWS_STORAGE" == "elasticsearch" ]; then
        cat <<EOF | sudo tee -a $SKYDIVE_CONFIG_FILE
  flow:
    backend: $SKYDIVE_FLOWS_STORAGE
EOF
    fi

    cat <<EOF | sudo tee -a $SKYDIVE_CONFIG_FILE
  topology:
    backend: $SKYDIVE_GRAPH_STORAGE
    probes:
$(get_probes_for_config $SKYDIVE_ANALYZER_PROBES)
EOF

    if [ "x$SKYDIVE_PUBLIC_INTERFACES" != "x" ]; then
        cat <<EOF | sudo tee -a $SKYDIVE_CONFIG_FILE
    fabric:
$(get_fabric_config)
EOF
    fi

    if [ "x$SKYDIVE_ANALYZER_LISTEN" != "x" ]; then
        cat <<EOF | sudo tee -a $SKYDIVE_CONFIG_FILE
  listen: $SKYDIVE_ANALYZER_LISTEN
EOF
    fi

    if [ "x$SKYDIVE_OVSDB_REMOTE_PORT" != "x" ]; then
        cat <<EOF | sudo tee -a $SKYDIVE_CONFIG_FILE
ovs:
  ovsdb: $SKYDIVE_OVSDB_REMOTE_PORT
  oflow:
    enable: true

EOF
    fi

    if is_service_enabled skydive-agent ; then
        if [ "x$SKYDIVE_OVSDB_REMOTE_PORT" != "x" ]; then
            sudo ovs-appctl -t ovsdb-server ovsdb-server/add-remote "ptcp:$SKYDIVE_OVSDB_REMOTE_PORT:127.0.0.1"
        fi
    fi
}

function start_skydive {
    if is_service_enabled skydive-agent ; then
        run_process skydive-agent "/usr/bin/skydive agent --conf $SKYDIVE_CONFIG_FILE" "root" "root"
    fi

    if is_service_enabled skydive-analyzer ; then
        if [ ${USE_ELASTICSEARCH} -eq 1 ]; then
            $TOP_DIR/pkg/elasticsearch.sh start
        fi
        run_process skydive-analyzer "/usr/bin/skydive analyzer --conf $SKYDIVE_CONFIG_FILE"
    fi
}

function stop_skydive {
    if is_service_enabled skydive-agent ; then
        stop_process skydive-agent
    fi

    if is_service_enabled skydive-analyzer ; then
        if [ ${USE_ELASTICSEARCH} -eq 1 ]; then
            $TOP_DIR/pkg/elasticsearch.sh stop
        fi
        stop_process skydive-analyzer
    fi
}

if is_service_enabled skydive-agent || is_service_enabled skydive-analyzer ; then
    if [[ "$1" == "stack" ]]; then
        if [[ "$2" == "pre-install" ]]; then
            pre_install_skydive
        elif [[ "$2" == "install" ]]; then
            install_skydive
        elif [[ "$2" == "post-config" ]]; then
            configure_skydive
            start_skydive
        fi
    fi

    if [[ "$1" == "unstack" ]]; then
        stop_skydive
        sudo rm $SKYDIVE_CONFIG_FILE
    fi
fi
