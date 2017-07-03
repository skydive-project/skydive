#!/bin/bash

export SKYDIVE_ANALYZERS=

WINDOW=0
AGENT=0
PREFIX=${PREFIX:-192.168.50}
VM_PREFIX=10.0 #/24
TEMP_DIR=${TEMP_DIR:-/tmp/skydive-scale}
ANALYZER_PORT=${ANALYZER_PORT:-8082}
CURR_ANALYZER_PORT=${CURR_ANALYZER_PORT:-8082}
GW_ADDR=$PREFIX.254
AGENT_STOCK=5

SKYDIVE_BIN=$( which skydive )
SKYDIVE_PATH=$( realpath $SKYDIVE_BIN )
SKYDIVE=${SKYDIVE:-$SKYDIVE_PATH}
TLS=${TLS:-false}
ELASTICSEARCH=${ELASTICSEARCH:-}
ETCD=${ETCD:-}
if [ -z "$ETCD" ]; then
	ETCD=$GW_ADDR:2379
	ETCD_EMBEDDED=true
else
	ETCD_EMBEDDED=false
fi

mkdir -p $TEMP_DIR

# create the main bridge which will connect all the analyzer/agent namespaces
function create_main_bridge() {
	echo Create central bridge

	sudo brctl addbr br-central
	sudo brctl stp br-central off

	sudo ip l set br-central up
	sudo ip a add $GW_ADDR/24 dev br-central
}

function delete_main_bridge() {
	echo Delete central bridge

	sudo ip link del br-central
}

# create a namespace with a veth connected to the main bridge
function create_host() {
	NAME=$1
	ADDR=$2

	echo Create host $NAME

	sudo ip netns add $NAME
	sudo ip netns exec $NAME ip link set lo up

	sudo ip link add $NAME-eth0 type veth peer name eth0 netns $NAME
	sudo ip link set $NAME-eth0 up

	sudo ip netns exec $NAME ip link set eth0 up
	sudo ip netns exec $NAME ip address add $ADDR/24 dev eth0
	sudo ip netns exec $NAME ip route add 0.0.0.0/0 via $GW_ADDR

	sudo brctl addif br-central $NAME-eth0
}

function generate_ssl_conf() {
	ANALYZER_NUM=$1
	AGENT_NUM=$2

	cat <<EOF > $TEMP_DIR/skydive-ssl.cnf
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
countryName_default = FR
stateOrProvinceName_default = Paris
localityName_default = Paris
organizationalUnitName_default	= Skydive Team
commonName = skydive
commonName_max	= 64

[ v3_req ]
# Extensions to add to a certificate request
basicConstraints = CA:TRUE
keyUsage = digitalSignature, keyEncipherment, keyCertSign
extendedKeyUsage = serverAuth,clientAuth
subjectAltName = @alt_names

[alt_names]
IP.1 = 127.0.0.1
IP.2 = $GW_ADDR
EOF

	for I in $( seq $ANALYZER_NUM ); do
		IDX=$(( $I + 2 ))
		echo IP.$IDX = $PREFIX.$I >> $TEMP_DIR/skydive-ssl.cnf
	done

	TOTAL_AGENT=$(( $AGENT_NUM + $AGENT_STOCK ))
	for I in $( seq $TOTAL_AGENT ); do
		IP=$(( $ANALYZER_NUM + $I ))
		IDX=$(( $ANALYZER_NUM + $I + 2))
		echo IP.$IDX = $PREFIX.$IP >> $TEMP_DIR/skydive-ssl.cnf
	done
}

function generate_tls_crt() {
	NAME=$1

	if [ -e $TEMP_DIR/$NAME.crt ]; then
		return
	fi

	sudo openssl genrsa -out $TEMP_DIR/$NAME.key 2048
	sudo chmod 400 $TEMP_DIR/$NAME.key
	yes '' | sudo openssl req -new -key $TEMP_DIR/$NAME.key -out $TEMP_DIR/$NAME.csr -subj "/CN=$NAME" -config $TEMP_DIR/skydive-ssl.cnf
	sudo openssl x509 -req -days 365 -signkey $TEMP_DIR/$NAME.key -in $TEMP_DIR/$NAME.csr -out $TEMP_DIR/$NAME.crt -extfile $TEMP_DIR/skydive-ssl.cnf -extensions v3_req
	sudo chmod 444 $TEMP_DIR/$NAME.crt
}

# create a agent with a nested ovs bridge. This agent will be connected to
# the analyzers. This function will also create fake nested namespaces simulating
# VMs connecting them to the local ovs.
function create_agent() {
	IDX=$1
	ADDR=$2
	ANALYZER_NUM=$3
	VM_NUM=$4

	NAME=agent-$IDX

	echo Create agent $NAME

	# nested ovs
	sudo ip netns exec $NAME ovsdb-tool create $TEMP_DIR/$NAME.db
	sudo ip netns exec $NAME ovsdb-server $TEMP_DIR/$NAME.db \
		--remote=punix:$TEMP_DIR/$NAME.sock \
	  --remote=db:Open_vSwitch,Open_vSwitch,manager_options \
	  --private-key=db:Open_vSwitch,SSL,private_key \
	  --certificate=db:Open_vSwitch,SSL,certificate \
	  --bootstrap-ca-cert=db:Open_vSwitch,SSL,ca_cert \
	  --log-file=$TEMP_DIR/$NAME-vswitchd.log \
	  -vsyslog:dbg -vfile:dbg --pidfile=$TEMP_DIR/$NAME.pid --detach
	sudo ip netns exec $NAME ovs-vswitchd unix:$TEMP_DIR/$NAME.sock \
		-vconsole:emer -vsyslog:err -vfile:info --mlockall --no-chdir \
		--log-file=$TEMP_DIR/$NAME-vswitchd.log \
		--pidfile=$TEMP_DIR/$NAME-vswitchd.pid --detach --monitor

		mkdir -p $TEMP_DIR/$NAME-netns

		# TLS if needed
		if [ $TLS = true ]; then
			AGENT_CRT=$TEMP_DIR/agent.crt
			AGENT_KEY=$TEMP_DIR/agent.key

			ANALYZER_CRT=$TEMP_DIR/analyzer.crt
		        ANALYZER_KEY=$TEMP_DIR/analyzer.key
		fi

		echo "analyzers:" > $TEMP_DIR/$NAME.yml
		for ANALYZER_I in $( seq $ANALYZER_NUM ); do
			PORT=$(( $ANALYZER_PORT + ($ANALYZER_I - 1) * 2 ))
			echo "  - $GW_ADDR:$PORT" >> $TEMP_DIR/$NAME.yml
		done

		cat <<EOF >> $TEMP_DIR/$NAME.yml
host_id: $NAME
ws_pong_timeout: 15
analyzer:
  X509_cert: $ANALYZER_CRT
  X509_key: $ANALYZER_KEY
agent:
  listen: 0.0.0.0:8081
  X509_cert: $AGENT_CRT
  X509_key: $AGENT_KEY
  X509_insecure: true
  topology:
    netlink:
      metrics_update: 5
    probes:
      - netlink
      - netns
      - ovsdb
  flow:
    probes:
      - gopacket
      - ovssflow
flow:
  expire: 600
  update: 5
netns:
  run_path: $TEMP_DIR/$NAME-netns
etcd:
  data_dir: $TEMP_DIR/$NAME-etcd
  servers:
    - http://$ETCD
ovs:
  ovsdb: unix://$TEMP_DIR/$NAME.sock
logging:
  level: DEBUG
EOF

	sudo -E screen -S skydive-stress -X screen -t skydive-agent $WINDOW ip netns exec $NAME sh -c "$SKYDIVE agent -c $TEMP_DIR/$NAME.yml 2>&1 | tee $TEMP_DIR/$NAME.log"
	WINDOW=$(( $WINDOW + 1 ))

	sudo ip netns exec $NAME ovs-vsctl --db=unix:$TEMP_DIR/$NAME.sock add-br $NAME
	sudo ip netns exec $NAME ovs-vsctl --db=unix:$TEMP_DIR/$NAME.sock set bridge $NAME rstp_enable=true

	# create a VM
	for VM_I in $( seq $VM_NUM ); do
		echo Create VM $NAME-vm$VM_I

		sudo ip netns add $NAME-vm$VM_I
		sudo ip netns exec $NAME-vm$VM_I ip l set lo up

		sudo ip netns exec $NAME ip link add vm$VM_I-eth0 type veth peer name eth0 netns $NAME-vm$VM_I
		sudo ip netns exec $NAME ip link set vm$VM_I-eth0 up

		sudo ip netns exec $NAME-vm$VM_I ip link set eth0 up
		sudo ip netns exec $NAME-vm$VM_I ip address add $VM_PREFIX.$IDX.$VM_I/16 dev eth0

		sudo ip netns exec $NAME ovs-vsctl --db=unix:$TEMP_DIR/$NAME.sock add-port $NAME vm$VM_I-eth0
	done

	touch $TEMP_DIR/$NAME.lock
}

function delete_host() {
	NAME=$1

	echo Delete Host $NAME

	sudo ovs-vsctl del-port br-central $NAME-eth0

	sudo ip link del $NAME-eth0
	sudo ip netns del $NAME
}

function delete_agent() {
	IDX=$1
	VM_NUM=$2

	NAME=agent-$IDX

	echo Delete agent $NAME

	sudo kill -9 `cat $TEMP_DIR/$NAME-vswitchd.pid`
	sudo kill -9 `cat $TEMP_DIR/$NAME.pid`

	for VM_I in $( seq $VM_NUM ); do
		sudo ip netns del $NAME-vm$VM_I
	done

	rm $TEMP_DIR/$NAME.lock
}

# create analyzer and connect it to the others. This function also create fabric
# part connecting the agents to a TOR.
function create_analyzer() {
	IDX=$1
	ANALYZER_NUM=$2
	AGENT_NUM=$3

	NAME=analyzer-$IDX

	echo Create analyzer $NAME

	mkdir -p $TEMP_DIR/$NAME-etcd

  echo "analyzers:" > $TEMP_DIR/$NAME.yml
	for ANALYZER_I in $( seq $ANALYZER_NUM ); do
		PORT=$(( $ANALYZER_PORT + ($ANALYZER_I - 1) * 2 ))
		echo "  - localhost:$PORT" >> $TEMP_DIR/$NAME.yml
	done

	STORAGE=""
	GRAPH="memory"
	if [ "x$ELASTICSEARCH" != "x" ]; then
		STORAGE="elasticsearch"
		GRAPH="elasticsearch"
	fi

	# TLS if needed
	if [ $TLS = true ]; then
		ANALYZER_CRT=$TEMP_DIR/analyzer.crt
		ANALYZER_KEY=$TEMP_DIR/analyzer.key

		AGENT_CRT=$TEMP_DIR/agent.crt
		AGENT_KEY=$TEMP_DIR/agent.key
	fi

	cat <<EOF >> $TEMP_DIR/$NAME.yml
host_id: $NAME
ws_pong_timeout: 15
etcd:
  embedded: $ETCD_EMBEDDED
  data_dir: $TEMP_DIR/$NAME-etcd
  servers:
    - http://$ETCD
  listen: $ETCD
storage:
  elasticsearch:
    host: $ELASTICSEARCH
logging:
  level: DEBUG
graph:
  backend: $GRAPH
flow:
  expire: 600
  update: 5
agent:
  X509_cert: $AGENT_CRT
  X509_key: $AGENT_KEY
  X509_insecure: true
analyzer:
  listen: 0.0.0.0:$CURR_ANALYZER_PORT
  X509_cert: $ANALYZER_CRT
  X509_key: $ANALYZER_KEY
  storage:
    backend: $STORAGE
  topology:
    fabric:
EOF

	CURR_ANALYZER_PORT=$(( $CURR_ANALYZER_PORT + 2 ))

	TOTAL_AGENT=$(( $AGENT_NUM + $AGENT_STOCK ))
  for AGENT_I in $( seq $TOTAL_AGENT ); do
		echo "      - TOR1 -> TOR1_PORT_$AGENT_I" >> $TEMP_DIR/$NAME.yml
		echo "      - TOR1_PORT_$AGENT_I -> *[Name=agent-$AGENT_I]/eth0" >> $TEMP_DIR/$NAME.yml
	done

	sudo -E screen -S skydive-stress -X screen -t skydive-analyzer $WINDOW sh -c "$SKYDIVE analyzer -c $TEMP_DIR/$NAME.yml 2>&1 | tee $TEMP_DIR/$NAME.log"
	WINDOW=$(( $WINDOW + 1 ))

	touch $TEMP_DIR/$NAME.lock
}

# create a full mesh tunnel between all the agent namespace so that VMs can ping
# others. Packet injector can be used as well.
function create_tunnel() {
	NAME=$1
	ADDR=$2
	AGENTS=$3

	echo Create tunnels...

	for AGENT_I in $( echo $AGENTS | sed "s/,/ /g" ); do
		if [ $ADDR != $AGENT_I ]; then
			echo "$ADDR -> $AGENT_I"

			sudo ip netns exec $NAME ovs-vsctl --db=unix:$TEMP_DIR/$NAME.sock add-port $NAME gre-$AGENT_I -- set interface gre-$AGENT_I type=gre options:remote_ip=$AGENT_I
		fi
	done
}

function ping() {
	SRC=$1
	DST=$2
	ARGS=${*: 3}

	IP_DST=$( sudo ip netns exec $DST ip a show eth0 | grep "inet " | awk '{print $2}' | cut -d '/' -f 1 )
	sudo ip netns exec $SRC ping $IP_DST $ARGS
}

# start all the services within a screen.
function start() {
	ANALYZER_NUM=$1
	AGENT_NUM=$2
	VM_NUM=$3

	if [ ! -e $TEMP_DIR/screen.lock ]; then
		sudo screen -dmS skydive-stress
		sudo screen -ls
		touch $TEMP_DIR/screen.lock
	fi

	# TLS if needed
	if [ $TLS = true ]; then
		generate_ssl_conf $ANALYZER_NUM $AGENT_NUM
		generate_tls_crt analyzer
		generate_tls_crt agent
	fi

	create_main_bridge

	start_inotify

	for ANALYZER_I in $( seq $ANALYZER_NUM ); do
		# if embedded only the first analyzer will be used as etcd server
		if [ $ANALYZER_I -gt 1 ]; then
			ETCD_EMBEDDED=false
		fi
		if [ ! -f $TEMP_DIR/analyzer-$ANALYZER_I.lock ]; then
			create_analyzer $ANALYZER_I $ANALYZER_NUM $AGENT_NUM
		fi
	done

	AGENTS=""
	for AGENT_I in $( seq $AGENT_NUM ); do
		if [ ! -f $TEMP_DIR/agent-$AGENT_I.lock ]; then
			create_host agent-$AGENT_I $PREFIX.$AGENT_I
			create_agent $AGENT_I $PREFIX.$AGENT_I $ANALYZER_NUM $VM_NUM
		fi

		if [ -z "$AGENTS" ]; then
			AGENTS=$PREFIX.$AGENT_I
		else
			AGENTS="$AGENTS,$PREFIX.$AGENT_I"
		fi
	done

	for AGENT_I in $( seq $AGENT_NUM ); do
		create_tunnel agent-$AGENT_I $PREFIX.$AGENT_I $AGENTS
	done
}

# start a inotify watcher in order to link fake nested namespace(VMs) to the
# correct agent. Basically creating a link between /var/run/netns and
# the correct run_path.
function start_inotify() {
	if [ -f $TEMP_DIR/inotify.lock ]; then
		return
	fi

	echo Start inotify netns watcher

	# create this folder if netns never created
	sudo mkdir -p /var/run/netns

	cat <<EOF > $TEMP_DIR/inotify.sh
inotifywait -m -r -e create -e delete /var/run/netns | while read PATH EVENT FOLDER; do
  if [[ \$FOLDER =~ agent-[0-9] ]]; then
    AGENT=\`echo -n \$FOLDER | /usr/bin/cut -d "-" -f -2\`
    NS=\`echo -n \$FOLDER | /usr/bin/cut -d "-" -f 3-\`

    case "\$EVENT" in
    CREATE)
      /usr/bin/sudo ln -s /var/run/netns/\$FOLDER $TEMP_DIR/\$AGENT-netns/\$NS
      ;;
    DELETE)
      /usr/bin/rm $TEMP_DIR/\$AGENT-netns/\$NS
      ;;
    esac
  fi
done
EOF

	sudo -E screen -S skydive-stress -X screen -t skydive-inotify $WINDOW bash -x $TEMP_DIR/inotify.sh
	touch $TEMP_DIR/inotify.lock
}

function stop_analyzer() {
	ANALYZER=$1

	NAME=analyzer-$ANALYZER
	sudo pkill -f $NAME.yml

	sudo rm $TEMP_DIR/$NAME.lock
}

function stop_agent() {
	AGENT=$1

	NAME=agent-$AGENT
	sudo pkill -f $NAME.yml

	sudo rm $TEMP_DIR/$NAME.lock
}

function stop() {
	ANALYZER_NUM=$1
	AGENT_NUM=$2
	VM_NUM=$3

	echo Stopping Analyzers...

	sudo -E screen -S skydive-stress -X quit
	rm $TEMP_DIR/screen.lock

	for AGENT_I in $( seq $AGENT_NUM ); do
		delete_agent $AGENT_I $VM_NUM
		delete_host agent-$AGENT_I
	done

	delete_main_bridge

	sudo find $TEMP_DIR  -mindepth 1 ! -name "*.log" -exec rm -rf {} \;
}

function check_dependencies() {
	if [ ! $(which inotifywait 2>/dev/null) ]; then
		echo "inotifywait not found !"
		exit 1
	fi

	if [ ! $(which realpath 2>/dev/null) ]; then
		echo "realpath not found !"
		exit 1
	fi
}

function usage() {
	echo -e "Usage:\t$0 start/stop <num analyzers> <num agents> <num vm> [elasticsearch address]"
	echo ""
	echo -e "      \t$0 ping <agent-1-vm1> <agent-2-vm1> [options]"
	echo -e "      \t$0 stop-analyzer <num>"
	exit 1
}

if [ -z "$1" -o "$1" == "help" ] ; then
	usage
fi

set -x

check_dependencies

if [ "$1" == "start" ]; then
	if [ -z "$4" ]; then
		usage
	fi
	start $2 $3 $4

	echo Go to : http://$GW_ADDR:$ANALYZER_PORT
elif [ "$1" == "stop" ]; then
	if [ -z "$4" ]; then
		usage
	fi
	stop $2 $3 $4
elif [ "$1" == "stop-analyzer" ]; then
	if [ -z "$2" ]; then
		usage
	fi

	stop_analyzer $2
elif [ "$1" == "stop-agent" ]; then
	if [ -z "$2" ]; then
		usage
	fi

	stop_agent $2
elif [ "$1" == "ping" ]; then
	if [ -z "$3" ]; then
		usage
	fi

	ping $2 $3 ${*: 4}
else
	usage
fi
