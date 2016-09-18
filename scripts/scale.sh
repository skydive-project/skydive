#!/bin/bash

# create/delete a test topology
# syntax:
#   ./scale.sh start <number>
#   ./scale.sh stop <number>
#

set -x

SKYDIVE=$( whereis skydive | cut -d ':' -f 2 )
WINDOW=0
AGENT=0
USER=$( whoami )
PREFIX=192.168.0

function create_ns() {
	NAME=$1
	IP=$2

	sudo ip netns add ${NAME}
	sudo ip netns exec ${NAME} ip link set lo up

	sudo ip link add ${NAME}-eth0 type veth peer name eth0 netns ${NAME}
	sudo ip link set ${NAME}-eth0 up

	sudo ip netns exec ${NAME} ip link set eth0 up
	sudo ip netns exec ${NAME} ip address add $IP/24 dev eth0

	sudo ovs-vsctl add-port br-scale ${NAME}-eth0
}

function delete_ns() {
	NAME=$1

	sudo ip link del ${NAME}-eth0
	sudo ip netns del ${NAME}
}

function start_analyzer() {
	NAME=$1
	IP=$2

	create_ns $NAME $IP

	sudo -E screen -S skydive-scale -X screen -t skydive-analyzer $WINDOW ip netns exec ${NAME} $SKYDIVE analyzer --listen=0.0.0.0:8082

	WINDOW=$(( $WINDOW + 1 ))
}

function stop_analyzer() {
	NAME=$1

	delete_ns $NAME
}

function start_capture() {
	sudo -E ip netns exec analyzer $SKYDIVE client capture create --gremlin "G.V().Has('Name', 'eth0')"
}

function start_agent() {
	NAME=$1
	IP=$2
	ANALYZER=$3

	create_ns $NAME $IP

	sudo -E screen -S skydive-scale -X screen -t skydive-agent $WINDOW ip netns exec ${NAME} $SKYDIVE agent ${AGENT_ARGS} --host-id=agent-$AGENT

	AGENT=$(( $AGENT + 1 ))
	WINDOW=$(( $WINDOW + 1 ))
}

function stop_agent() {
	NAME=$1

	delete_ns $NAME
}

function random_ping() {
	NAME=$1
	IP=$2

	sudo ip netns exec ${NAME} ping -c 5 $IP
}

function start_locust() {
	NUM=$1
	NAME=locust
	IP=$PREFIX.251

	rm /tmp/skydive-locust.py
	cat<<'EOF'>> /tmp/skydive-locust.py
	from locust import HttpLocust, TaskSet, task

	class UserBehavior(TaskSet):
	@task(1)
	def index(self):
	self.client.get("/")

	class WebsiteUser(HttpLocust):
	task_set = UserBehavior
	min_wait=5000
	max_wait=9000
EOF
	create_ns $NAME $IP

	sudo -E screen -S skydive-scale -X screen -t http-server $WINDOW ip netns exec ${NAME} su - $USER -c "python -m SimpleHTTPServer 8080"
	sudo -E screen -S skydive-scale -X screen -t locust-master $WINDOW ip netns exec ${NAME} su - $USER -c "locust -f /tmp/skydive-locust.py --master --host=http://$IP:8080"

	WINDOW=$(( $WINDOW + 1 ))

	for i in $( seq $NUM ); do
		sudo -E screen -S skydive-scale -X screen -t locust-slave $WINDOW ip netns exec agent-$i su - $USER -c "locust -f /tmp/skydive-locust.py --slave --master-host=$IP"

		WINDOW=$(( $WINDOW + 1 ))
	done
}

function stop_locust() {
	NAME=locust

	delete_ns $NAME

	sudo killall -9 locust
}

function add_gateway() {
	sudo ovs-vsctl add-port br-scale gateway -- set interface gateway type=internal
	sudo ip l set gateway up
	sudo ip a add $PREFIX.254/24 dev gateway
}

function start() {
	NUM=$1

	sudo rm -rf /tmp/skydive-etcd

	sudo -E screen -dmS skydive-scale

	sudo ovs-vsctl add-br br-scale

	start_analyzer analyzer $PREFIX.250

	for i in $( seq $NUM ); do
		start_agent agent-$i $PREFIX.$i $PREFIX.250:8082
	done

	sleep 1
	start_capture

	start_locust $NUM

	add_gateway
}

function stop() {
	NUM=$1

	sudo -E screen -S skydive-scale -X quit

	sudo ovs-vsctl del-br br-scale

	stop_analyzer analyzer

	stop_locust

	for i in $( seq $NUM ); do
		stop_agent agent-$i
	done
}

if [ -z "$2" ]; then
	echo "Usage: $0 start/stop <number>"
	exit 1
fi

if [ "$1" == "start" ]; then
	start $2
else
	stop $2
fi
