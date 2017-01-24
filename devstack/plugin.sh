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

# golang version. Skydive needs at least version 1.6
GO_VERSION=${GO_VERSION:-1.6}

# GOPATH where the go src, pkgs are installed
GOPATH=/opt/stack/go

# Address on which skydive analyzer process listens for connections.
# Must be in ip:port format
SKYDIVE_ANALYZER_LISTEN=${SKYDIVE_ANALYZER_LISTEN:-$SERVICE_HOST:8082}

# Inform the agent about the address on which analyzers are listening
SKYDIVE_AGENT_ANALYZERS=${SKYDIVE_AGENT_ANALYZERS:-$SKYDIVE_ANALYZER_LISTEN}

# Configure the skydive agent with the etcd server address
SKYDIVE_AGENT_ETCD=${SKYDIVE_AGENT_ETCD:-http://$SERVICE_HOST:2379}

# ip:port address on which skydive agent listens for connections.
SKYDIVE_AGENT_LISTEN=${SKYDIVE_AGENT_LISTEN:-"127.0.0.1:8081"}

# The path for the generated skydive configuration file
SKYDIVE_CONFIG_FILE=${SKYDIVE_CONFIG_FILE:-"/tmp/skydive.yaml"}

# List of agent probes to be used by the agent
SKYDIVE_AGENT_PROBES=${SKYDIVE_AGENT_PROBES:-"netlink netns ovsdb neutron"}

# List of analyzer probes to be used by the analyzer
SKYDIVE_ANALYZER_PROBES=${SKYDIVE_ANALYZER_PROBES:-""}

# Remote port for ovsdb server.
SKYDIVE_OVSDB_REMOTE_PORT=${SKYDIVE_OVSDB_REMOTE_PORT:-}

# Default log level
SKYDIVE_LOGLEVEL=${SKYDIVE_LOGLEVEL:-INFO}

# Storage used by the analyzer to store flows
SKYDIVE_STORAGE=${SKYDIVE_STORAGE:-"elasticsearch"}

# List of public interfaces for the agents to register in fabric
# ex: "devstack1/eth0 devstack2/eth1"
if [ "x$PUBLIC_INTERFACE" != "x" ]; then
    SKYDIVE_PUBLIC_INTERFACES=${SKYDIVE_PUBLIC_INTERFACES:-$LOCAL_HOSTNAME/$PUBLIC_INTERFACE}
fi

ELASTICSEARCH_BASE_URL=https://download.elastic.co/elasticsearch/release/org/elasticsearch/distribution
ELASTICSEARCH_VERSION=2.3.1

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
            wget ${ELASTICSEARCH_BASE_URL}/deb/elasticsearch/${ELASTICSEARCH_VERSION}/elasticsearch-${ELASTICSEARCH_VERSION}.deb \
                -O ${TOP_DIR}/files/elasticsearch-${ELASTICSEARCH_VERSION}.deb
        elif is_fedora; then
            wget ${ELASTICSEARCH_BASE_URL}/rpm/elasticsearch/${ELASTICSEARCH_VERSION}/elasticsearch-${ELASTICSEARCH_VERSION}.rpm \
                -O ${TOP_DIR}/files/elasticsearch-${ELASTICSEARCH_VERSION}.noarch.rpm
        fi
    fi
}

function pre_install_skydive {
    install_protoc
    install_go
    if is_service_enabled skydive-analyzer; then
        download_elasticsearch
        export ELASTICSEARCH_VERSION
        $TOP_DIR/pkg/elasticsearch.sh install
    fi
}

function install_skydive {
    if [ ! -f $GOPATH/bin/skydive ]; then
        if is_fedora ; then
            install_package libpcap-devel
        else
            install_package libpcap-dev
        fi
        SKYDIVE_SRC=$GOPATH/src/github.com/skydive-project
        mkdir -p $SKYDIVE_SRC
	if [ ! -d $SKYDIVE_SRC/skydive ]; then
		mv $DEST/skydive $SKYDIVE_SRC/
		ln -s $SKYDIVE_SRC/skydive $DEST/skydive
	fi
        cd $SKYDIVE_SRC/skydive
        make install
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
        echo "      - PORT_${host} -> *[Type=host, Name=${host}]/${intf}"
    done
}

function configure_skydive {
    cat > $SKYDIVE_CONFIG_FILE <<- EOF
auth:
  type: keystone

logging:
  default: $SKYDIVE_LOGLEVEL

openstack:
  auth_url: ${KEYSTONE_AUTH_PROTOCOL}://${KEYSTONE_AUTH_HOST}:${KEYSTONE_AUTH_PORT}/v3
  username: admin
  password: $ADMIN_PASSWORD
  tenant_name: admin
  region_name: RegionOne
  domain_name: $SERVICE_DOMAIN_NAME

etcd:
  servers:
    - $SKYDIVE_AGENT_ETCD
  data_dir: /tmp/skydive-etcd

graph:
  backend: elasticsearch

agent:
  analyzer_username: admin
  analyzer_password: $ADMIN_PASSWORD
  analyzers: $SKYDIVE_AGENT_ANALYZERS
  listen: $SKYDIVE_AGENT_LISTEN
  flow:
    probes:
      - ovssflow
      - gopacket
  topology:
    probes:
$(get_probes_for_config $SKYDIVE_AGENT_PROBES)

analyzer:
  storage: $SKYDIVE_STORAGE
  topology:
    probes:
$(get_probes_for_config $SKYDIVE_ANALYZER_PROBES)
EOF

    if [ "x$SKYDIVE_PUBLIC_INTERFACES" != "x" ]; then
        cat >> $SKYDIVE_CONFIG_FILE <<- EOF
    fabric:
$(get_fabric_config)
EOF
    fi

    if [ "x$SKYDIVE_ANALYZER_LISTEN" != "x" ]; then
        cat >> $SKYDIVE_CONFIG_FILE <<- EOF
  listen: $SKYDIVE_ANALYZER_LISTEN
EOF
    fi

    if [ "x$SKYDIVE_OVSDB_REMOTE_PORT" != "x" ]; then
        cat >> $SKYDIVE_CONFIG_FILE <<- EOF
ovs:
  ovsdb: $SKYDIVE_OVSDB_REMOTE_PORT

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
        run_process skydive-agent "sudo $GOPATH/bin/skydive agent --conf $SKYDIVE_CONFIG_FILE"
    fi

    if is_service_enabled skydive-analyzer ; then
        $TOP_DIR/pkg/elasticsearch.sh start
        run_process skydive-analyzer "$GOPATH/bin/skydive analyzer --conf $SKYDIVE_CONFIG_FILE"
    fi
}

function stop_skydive {
    if is_service_enabled skydive-agent ; then
        stop_process skydive-agent
    fi

    if is_service_enabled skydive-analyzer ; then
        $TOP_DIR/pkg/elasticsearch.sh stop
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
        rm $SKYDIVE_CONFIG_FILE
    fi
fi
