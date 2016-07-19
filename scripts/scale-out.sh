#!/bin/bash

if [ -z "$2" ]; then
	echo "Usage: scale-out.sh <num> <conf file>"
	exit 1
fi

screen -dmS skydive-scale

for i in $(seq $1); do
	port=$(( 40000 + $i ))
	screen -S skydive-scale -X screen -t skydive-agent $i /home/safchain/code/gocode/bin/skydive agent -c $2 --listen $port --host_id=node-$i
	sleep 2
	echo started $i/$1
done

echo All started....
read

screen -S skydive-scale -X quit
