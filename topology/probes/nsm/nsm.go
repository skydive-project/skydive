/*
 * Copyright (C) 2018 Orange
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package nsm

import (
	"context"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	cc "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/crossconnect"
	localconn "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	"github.com/networkservicemesh/networkservicemesh/k8s/pkg/networkservice/clientset/versioned"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// Probe ...
type Probe struct {
	common.RWMutex
	graph.DefaultGraphListener
	g     *graph.Graph
	state int64
	nsmds map[string]*grpc.ClientConn

	// slice of connections to track existing links between inodes
	connections []connection
}

// NewNsmProbe ...
func NewNsmProbe(g *graph.Graph) (*Probe, error) {
	logging.GetLogger().Debug("creating probe")
	probe := &Probe{
		g:     g,
		nsmds: make(map[string]*grpc.ClientConn),
	}
	atomic.StoreInt64(&probe.state, common.StoppedState)
	return probe, nil
}

// Start ...
func (p *Probe) Start() {
	p.g.AddEventListener(p)
	go p.run()
}

// Stop ....
func (p *Probe) Stop() {
	atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState)
	p.g.RemoveEventListener(p)
	for _, conn := range p.nsmds {
		conn.Close()
	}
}

func (p *Probe) run() {
	atomic.StoreInt64(&p.state, common.RunningState)

	// check if CRD is installed
	config, err := rest.InClusterConfig()
	if err != nil {
		logging.GetLogger().Errorf("Unable to get in cluster config, attempting to fall back to kubeconfig", err)
		return
	}

	logging.GetLogger().Debugf("NSM: getting NSM client")
	// Initialize clientset
	nsmClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		logging.GetLogger().Errorf("Unable to initialize nsmd-k8s", err)
		return
	}

	result, err := nsmClientSet.Networkservicemesh().NetworkServiceManagers("default").List(metav1.ListOptions{})
	if err != nil {
		logging.GetLogger().Errorf("Unable to find NSMs, are they running?", err)
		return
	}
	for _, mgr := range result.Items {
		//TODO: loop each nsmd servers monitoring in dedicated goroutines
		if _, ok := p.nsmds[mgr.Status.URL]; !ok {

			logging.GetLogger().Infof("Found nsmd: %s at %s", mgr.Name, mgr.Status.URL)
			go p.monitorCrossConnects(mgr.Status.URL)
		}
	}
	for atomic.LoadInt64(&p.state) == common.RunningState {
		time.Sleep(1 * time.Second)
	}
}

func (p *Probe) monitorCrossConnects(url string) {
	var err error
	p.nsmds[url], err = dial(context.Background(), "tcp", url)
	if err != nil {
		logging.GetLogger().Errorf("NSM: unable to create grpc dialer, error: %+v", err)
		return
	}

	client := cc.NewMonitorCrossConnectClient(p.nsmds[url])
	//TODO: grpc is automagically trying to reconnect
	// better understand the process to handle corner cases
	stream, err := client.MonitorCrossConnects(context.Background(), &empty.Empty{})
	if err != nil {
		logging.GetLogger().Errorf("NSM: unable to connect to grpc server, error: %+v.", err)
		return
	}

	for {
		logging.GetLogger().Debugf("NSM: waiting for events")
		event, err := stream.Recv()
		if err != nil {
			logging.GetLogger().Errorf("Error: %+v.", err)
			return
		}

		logging.GetLogger().Debugf("NSM: received monitoring event of type %s", event.Type)
		for _, cconn := range event.GetCrossConnects() {
			cconnStr := proto.MarshalTextString(cconn)

			lSrc := cconn.GetLocalSource()
			rSrc := cconn.GetRemoteSource()
			lDst := cconn.GetLocalDestination()
			rDst := cconn.GetRemoteDestination()

			switch {
			case lSrc != nil && rSrc == nil && lDst != nil && rDst == nil:
				logging.GetLogger().Debugf("NSM: Got local to local CrossConnect Msg \n%s", cconnStr)
				p.onConnLocalLocal(event.GetType(), cconn) //cconn.GetId(), cconn.GetPayload(), lSrc, lDst)
			case lSrc == nil && rSrc != nil && lDst != nil && rDst == nil:
				logging.GetLogger().Debugf("NSM: Got remote to local CrossConnect Msg \n%s", cconnStr)
				p.onConnRemoteLocal(event.GetType(), cconn)
			case lSrc != nil && rSrc == nil && lDst == nil && rDst != nil:
				logging.GetLogger().Debugf("NSM: Got local to remote CrossConnect Msg \n%s", cconnStr)
				p.onConnLocalRemote(event.GetType(), cconn)
			default:
				logging.GetLogger().Errorf("NSM: Error parsing CrossConnect \n%s", cconnStr)
			}
		}
	}
}

func (p *Probe) onConnLocalLocal(t cc.CrossConnectEventType, conn *cc.CrossConnect) {
	srcInode, err := getLocalInode(conn.GetLocalSource())
	if err != nil {
		return
	}
	dstInode, err := getLocalInode(conn.GetLocalDestination())
	if err != nil {
		return
	}

	p.Lock()
	defer p.Unlock()
	if t != cc.CrossConnectEventType_DELETE {
		l := &localConnectionPair{
			ID: conn.GetId(),
			baseConnectionPair: baseConnectionPair{
				payload:  conn.GetPayload(),
				srcInode: srcInode,
				dstInode: dstInode,
				src:      conn.GetLocalSource(),
				dst:      conn.GetLocalDestination(),
			},
		}
		logging.GetLogger().Infof("adding link %+v", l)
		p.connections = append(p.connections, l)
		p.g.Lock()
		l.AddEdge(p.g)
		p.g.Unlock()
	} else {
		for i := range p.connections {
			l, ok := p.connections[i].(*localConnectionPair)
			if !ok {
				continue
			}
			if l.srcInode == srcInode && l.dstInode == dstInode {
				p.g.Lock()
				l.DelEdge(p.g)
				p.g.Unlock()
				//move last elem to position i and truncate the slice
				p.connections[i] = p.connections[len(p.connections)-1]
				p.connections[len(p.connections)-1] = nil
				p.connections = p.connections[:len(p.connections)-1]
				break
			}
		}
	}
}

func (p *Probe) onConnLocalRemote(t cc.CrossConnectEventType, conn *cc.CrossConnect) {
	p.Lock()
	defer p.Unlock()
	var c *remoteConnectionPair
	var i int
	c, i = p.getConnectionWithRemote(conn.GetRemoteDestination().GetId())
	if t != cc.CrossConnectEventType_DELETE {
		if c == nil {
			c = new(remoteConnectionPair)
			c.remote = conn.GetRemoteDestination()
			p.connections = append(p.connections, c)
			c.src = conn.GetLocalSource()
			c.srcID = conn.GetId()
			//TODO: we erase previous payload if exist
			c.payload = conn.GetPayload()

		} else {
			c.src = conn.GetLocalSource()
			c.srcID = conn.GetId()
			//TODO: we erase previous payload if exist
			c.payload = conn.GetPayload()
			p.g.Lock()
			c.AddEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if c == nil {
			logging.GetLogger().Warning("NSM: received cross connect delete event for a connection that does not exist")
			return
		}

		p.g.Lock()
		c.DelEdge(p.g)
		p.g.Unlock()

		// Delete the link only if dst is also empty
		if c.dst == nil {
			p.connections[i] = p.connections[len(p.connections)-1]
			p.connections[len(p.connections)-1] = nil
			p.connections = p.connections[:len(p.connections)-1]

		} else {
			c.src = nil
		}
	}
}

func (p *Probe) onConnRemoteLocal(t cc.CrossConnectEventType, conn *cc.CrossConnect) {
	p.Lock()
	defer p.Unlock()
	var c *remoteConnectionPair
	var i int
	c, i = p.getConnectionWithRemote(conn.GetRemoteSource().GetId())
	if t != cc.CrossConnectEventType_DELETE {
		if c == nil {
			c = new(remoteConnectionPair)
			c.remote = conn.GetRemoteSource()
			p.connections = append(p.connections, c)
			c.dst = conn.GetLocalDestination()
			c.dstID = conn.GetId()
			//TODO: compare payloads, they should be identical
			c.payload = conn.GetPayload()

		} else {
			c.dst = conn.GetLocalDestination()
			c.dstID = conn.GetId()
			//TODO: compare payloads, they should be identical
			c.payload = conn.GetPayload()
			p.g.Lock()
			c.AddEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if c == nil {
			logging.GetLogger().Warning("NSM: received cross connect delete event for a connection that does not exist")
			return

		}

		p.g.Lock()
		c.DelEdge(p.g)
		p.g.Unlock()

		// Delete the link only if src is also empty
		if c.GetSource() == nil {
			p.connections[i] = p.connections[len(p.connections)-1]
			p.connections[len(p.connections)-1] = nil
			p.connections = p.connections[:len(p.connections)-1]

		} else {
			c.dst = nil
		}
	}
}

// OnNodeAdded event
// We assume skydive has locked the graph before calling this function
func (p *Probe) OnNodeAdded(n *graph.Node) {
	p.Lock()
	defer p.Unlock()
	if i, err := n.GetFieldInt64("Inode"); err == nil {
		// Find connections with matching inode
		logging.GetLogger().Debugf("NSM: node added with inode: %v", i)
		c, err := p.getConnectionWithInode(i)
		if err != nil {
			logging.GetLogger().Errorf("NSM: error retreiving connections with inodes : %v", err)
			return
		}
		if len(c) == 0 {
			logging.GetLogger().Debugf("NSM: no connection with inode %v", i)
			return
		}

		for i := range c {
			c[i].AddEdge(p.g)
		}
	}
}

// OnNodeDeleted event
// We assume skydive has locked the graph before calling this function
func (p *Probe) OnNodeDeleted(n *graph.Node) {
	if i, err := n.GetFieldInt64("Inode"); err == nil {
		logging.GetLogger().Infof("NSM: node deleted with inode %v", i)
	}
	// If a graph node has been deleted, skydive should have automatically deleted egdes that was having this node as source or dest
	// TODO: Consider removing the corresponding connection from the connection list,
	// but it should be done by corresponding CrossConnect DELETE events received from nsmds
}

func (p *Probe) getConnectionWithInode(inode int64) ([]connection, error) {
	var c []connection
	for i := range p.connections {
		srcInode, err := getLocalInode(p.connections[i].GetSource())
		if err != nil {
			return nil, err
		}
		dstInode, err := getLocalInode(p.connections[i].GetDest())
		if err != nil {
			return nil, err
		}

		if srcInode == inode || dstInode == inode {
			c = append(c, p.connections[i])
		}
	}
	return c, nil
}

// return the connection from p.connections
// the probe has to be locked before calling this function
func (p *Probe) getConnectionWithRemote(id string) (*remoteConnectionPair, int) {
	for i := range p.connections {
		c, ok := p.connections[i].(*remoteConnectionPair)
		if !ok {
			continue
		}

		if c.remote.GetId() == id {
			return c, i
		}
	}
	return nil, 0
}

func dial(ctx context.Context, network string, address string) (*grpc.ClientConn, error) {
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.Dial(network, addr)
		}),
	)
	return conn, err
}

//TODO: consider moving this function to nsm helper functions in the local/connection package
func getLocalInode(conn *localconn.Connection) (int64, error) {
	inodeStr := conn.Mechanism.Parameters[localconn.NetNsInodeKey]
	inode, err := strconv.ParseInt(inodeStr, 10, 64)
	if err != nil {
		logging.GetLogger().Errorf("NSM: error converting inode %s to int64", inodeStr)
		return 0, err
	}
	return inode, nil
}
