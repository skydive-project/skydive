#!/bin/bash

success() {
    msg=$1

    echo "ok $current - $msg"

    success=$(( $success + 1 ))
    current=$(( $current + 1 ))
}

failed() {
    msg=$1

    echo "ko $current - $msg"

    failed=$(( $failed + 1 ))
    errors+=" $current"
    current=$(( $current + 1 ))
}

join_by() { local d=$1; shift; echo -n "$1"; shift; printf "%s" "${@/#/$d}"; }

test_agents() {
    expected=$1

    present=$( $skydive client status | jq '.Agents | length' )
    if [ "$present" != "$expected" ]; then
        failed "expected $expected found $present via status"
        
    else 
        success "found $present agent(s) via status"
    fi

    present=$( $skydive client query "G.V().Has('Type', 'host')" | jq '. | length' )
    if [ "$present" != "$expected" ]; then
        failed "expected $expected found $present via topology"
    else 
        success "found $present agent(s) via topology"
    fi
}

test_capture() {
    gremlin="G.V().Has('Type', 'host').Out().Has('State', 'UP').HasKey('IPV4')"

    intf=$( $skydive client query "$gremlin" | jq -r .[0].ID )
    if [ -z "intf" ]; then
        failed "no interface available"
    else 
        success "found an available interface for capture"
    fi

    result=$( $skydive client capture create --name capture-test --gremlin "$gremlin" )
    if [ $? -ne 0 ]; then
        failed "capture create request error"
    else
        success "capture creation succeed"
    fi
    uuid=$( echo -n "$result" | jq -r .UUID )

    # wait a bit and generate flows
    for i in {1..10}; do
        count=$( $skydive client query "G.V().Has('Capture.State', 'active').Count()" )
         if [ ! -z "$count" ] && [ $count -ne 0 ]; then
            break
         fi
         sleep 5
    done

    ips=$( $skydive client query "$gremlin.Values('IPV4')" | jq -r '.[] | @sh' )
    for ip in $ips; do
        ping -c 5 $( echo $ip | tr -d "'" | cut -d '/' -f 1 ) > /dev/null
    done

    count=$( $skydive client query "G.Flows().Count()" )
    if [ $? -ne 0 ]; then
        intfs=$( $skydive client query "$gremlin" | jq .[].Metadata.Name )
        failed "flow request error, interfaces: $intfs"
    else
        success "flow request succeed"
    fi
    
    if [ -z "$count" ] || [ $count -eq 0 ]; then
        intfs=$( $skydive client query "$gremlin" | jq .[].Metadata.Name )
        failed "no flow found, interfaces: $intfs"
    else
        success "$count flows found"
    fi
    
    $( $skydive client capture delete $uuid )
    if [ $? -ne 0 ]; then
        failed "capture delete error"
    else
        success "capture deletion succeed"
    fi
}

test_injection() {
    intf=$( $skydive client query "G.V().Has('Type', 'host').Out().HasKey('MAC').Has('State', 'UP', 'MAC', NE('')).HasKey('IPV4')" | jq -r .[0].ID )
    if [ -z "intf" ]; then
        failed "no interface available"
    else 
        success "found an available interface for capture"
    fi

    result=$( $skydive client capture create --name capture-test --gremlin "G.V('$intf')" )
    if [ $? -ne 0 ]; then
        failed "capture create request error"
    else
        success "capture created"
    fi
    capture_uuid=$( echo -n "$result" | jq -r .UUID )

    # wait a bit and generate flows
    for i in {1..10}; do
        count=$( $skydive client query "G.V().Has('Capture.State', 'active').Count()" )
         if [ ! -z "$count" ] && [ $count -ne 0 ]; then
            break
         fi
         sleep 5
    done

    result=$( $skydive client inject-packet create --count 30 --interval 1000 --id 9999 --src "G.V('$intf')" --dst "G.V('$intf')" )
    if [ $? -ne 0 ]; then
        failed "packet injection create request error"
    else
        success "packet injection creation succeed"
    fi
    injection_uuid=$( echo -n "$result" | jq -r .UUID )

    # wait a bit for flows
    sleep 5

    count=$( $skydive client query "G.V('$intf').Flows().Has('ICMP.ID', 9999).Count()" )
    if [ $? -ne 0 ]; then
        failed "flow request error, interface: $intf"
    else
        success "flow request succeed"
    fi
    
    if [ -z "$count" ] || [ $count -eq 0 ]; then
        failed "no flow found, interface: $intf"
    else
        success "$count flows found"
    fi
    
    $( $skydive client capture delete $capture_uuid )
    if [ $? -ne 0 ]; then
        failed "capture delete error"
    else
        success "capture deletion succeed"
    fi

    $( $skydive client inject-packet delete $injection_uuid )
    if [ $? -ne 0 ]; then
        failed "packet injection delete error"
    else
        success "packet injection deletion succeed"
    fi
}

test_ovs() {
    count=$( $skydive client query "G.V().HasKey('Ovs')" | jq '. | length' )
    if [ $? -ne 0 ]; then
        failed "ovs node request error"
    else
        success "ovs node request succeed"
    fi

    if [ -z "$count" ] || [ $count -eq 0 ]; then
        failed "no ovs node found"
    else
        success "$count ovs node found"
    fi
}

test_neutron() {
    count=$( $skydive client query "G.V().HasKey('Neutron')" | jq '. | length' )
    if [ $? -ne 0 ]; then
        failed "neutron node request error"
    else
        success "neutron node request succeed"
    fi

    if [ -z "$count" ] || [ $count -eq 0 ]; then
        failed "no neutron node found"
    else
        success "$count neutron node found"
    fi
}


usage() {
    echo "$0 -a <analyzer address>"
    echo "options:"
    echo "  -u, --username      analyzer username"
    echo "  -p, --password      analyzer password"
    echo "  -e, --agents <num>  number of expected agents"
    echo "  -c, --capture       test flow capture"
    echo "  -i, --injection     test packet injection"
    echo "  -o, --ovs           test OpenvSwitch reported"
    echo "  -n, --neutron       test OpenStack Neutron reported"
}

# tests status
tests=0
current=1
success=0
failed=0
errors=()

agents=0
capture=0
injection=0
ovs=0
neutron=0

while [ "$1" != "" ]; do
    case $1 in
        -a | --analyzer )           
            shift
            analyzer=$1
            ;;
        -u | --username )
            shift    
            username=$1
            ;;
        -p | --password )
            shift    
            password=$1
            ;;
        -e | --agents )
            shift
            agents=$1
            tests=$(( $tests + 2 ))
            ;;
        -c | --capture )
            capture=1
            tests=$(( $tests + 5 ))
            ;;
        -i | --injection )
            injection=1
            tests=$(( $tests + 7 ))
            ;;
        -o | --ovs )
            ovs=1
            tests=$(( $tests + 2 ))
            ;;
        -n | --neutron )
            neutron=1
            tests=$(( $tests + 2 ))
            ;;
        -h | --help )           
            usage
            exit
            ;;
        * ) 
            usage
            exit 1
    esac
    shift
done

if [ -z "$analyzer" ]; then
    usage
    exit 1
fi

# authentication
if [ ! -z "$username" ]; then
    auth_opts="--username $username --password $password"
fi

skydive="skydive --analyzer=$analyzer $auth_opts"
echo $skydive

if [ $tests -eq 0 ]; then
    exit
fi

# run test suite
echo "1..$tests"

if [ ! -z "$agents" ]; then
    test_agents $agents
fi

if [ $capture -ne 0 ]; then
    test_capture
fi

if [ $injection -ne 0 ]; then
    test_injection
fi

if [ $ovs -ne 0 ]; then
    test_ovs
fi

if [ $neutron -ne 0 ]; then
    test_neutron
fi

# summarize
if [ ${#errors[@]} -ne 0 ]; then
    echo "Failed tests" $( join_by ", " $errors )
fi

echo "Failed $failed/$tests tests, "$( bc <<< "scale=2; $success/$tests*100" )"% okay"

if [ $failed -ne 0 ]; then
    exit 1
fi
