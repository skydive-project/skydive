// +build !windows

/*
 * Copyright (C) 2019 Orange
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package nsm

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	cc "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/crossconnect"
	localconn "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	remoteconn "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/connection"
	v1 "github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis/networkservice/v1"
	"github.com/networkservicemesh/networkservicemesh/k8s/pkg/networkservice/clientset/versioned"
	"github.com/networkservicemesh/networkservicemesh/k8s/pkg/networkservice/informers/externalversions"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"google.golang.org/grpc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const nsmResource = "networkservicemanagers"

var informerStopper chan struct{}

// Probe represents the NSM probe
type Probe struct {
	common.RWMutex
	graph.DefaultGraphListener
	g     *graph.Graph
	state int64
	nsmds map[string]*grpc.ClientConn

	// slice of connections to track existing links between inodes
	connections []connection
}

// NewNsmProbe creates the Probe
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
	atomic.StoreInt64(&p.state, common.RunningState)

	// check if CRD is installed
	config, err := rest.InClusterConfig()
	if err != nil {
		logging.GetLogger().Errorf("Unable to get in K8S cluster config, make sure the analyzer is running inside a K8S pod", err)
		return
	}

	logging.GetLogger().Debugf("NSM: getting NSM client")
	// Initialize clientset
	nsmClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		logging.GetLogger().Errorf("Unable to initialize the NSM probe", err)
		return
	}

	factory := externalversions.NewSharedInformerFactory(nsmClientSet, 0)
	genericInformer, err := factory.ForResource(v1.SchemeGroupVersion.WithResource(nsmResource))
	if err != nil {
		logging.GetLogger().Errorf("Unable to create the K8S cache factory", err)
		return
	}

	informer := genericInformer.Informer()
	informerStopper := make(chan struct{})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			nsm := obj.(*v1.NetworkServiceManager)
			logging.GetLogger().Infof("New NSMgr Added: %v", nsm)
			go p.monitorCrossConnects(nsm.Status.URL)
		},
	})
	go informer.Run(informerStopper)
}

// Stop ....
func (p *Probe) Stop() {
	p.Lock()
	defer p.Unlock()
	if !atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		return
	}
	p.g.RemoveEventListener(p)
	for _, conn := range p.nsmds {
		conn.Close()
	}
	if informerStopper != nil {
		close(informerStopper)
	}
}

func (p *Probe) getNSMgrClient(url string) (cc.MonitorCrossConnectClient, error) {
	conn, err := dial(context.Background(), "tcp", url)
	if err != nil {
		logging.GetLogger().Errorf("NSM: unable to create grpc dialer, error: %+v", err)
		return nil, err
	}
	p.Lock()
	defer p.Unlock()
	p.nsmds[url] = conn
	return cc.NewMonitorCrossConnectClient(p.nsmds[url]), nil
}

func (p *Probe) monitorCrossConnects(url string) {
	client, err := p.getNSMgrClient(url)
	if err != nil {
		logging.GetLogger().Errorf("NSM: unable to connect to grpc server, error: %+v.", err)
		return
	}
	stream, err := client.MonitorCrossConnects(context.Background(), &empty.Empty{})
	if err != nil {
		logging.GetLogger().Errorf("NSM: unable to stream the grpc connection, error: %+v.", err)
		return
	}

	for {
		logging.GetLogger().Debugf("NSM: waiting for events")
		event, err := stream.Recv()
		if err != nil {
			logging.GetLogger().Errorf("Error: %+v.", err)
			return
		}

		logging.GetLogger().Debugf("NSM: received monitoring event of type %s from %s", event.GetType(), url)
		for _, cconn := range event.GetCrossConnects() {
			cconnStr := proto.MarshalTextString(cconn)

			lSrc := cconn.GetLocalSource()
			rSrc := cconn.GetRemoteSource()
			lDst := cconn.GetLocalDestination()
			rDst := cconn.GetRemoteDestination()

			// we filter out UPDATE messages in DOWN state
			// NSM will try to auto heal this kind of connection
			// If the state goes DOWN because of a deletion of the cross connect
			// we'll receive a DELETE message
			isDelMsg := event.GetType() == cc.CrossConnectEventType_DELETE
			switch {
			case lSrc != nil && rSrc == nil && lDst != nil && rDst == nil:
				logging.GetLogger().Debugf("NSM: Got local to local CrossConnect with id %s", cconnStr)
				if !isDelMsg && (lSrc.GetState() == localconn.State_DOWN || lDst.GetState() == localconn.State_DOWN) {
					logging.GetLogger().Debugf("NSM: one connection of the cross connect %s is in DOWN state, don't affect the skydive connections", cconn.GetId())
					continue
				}
				p.onConnLocalLocal(event.GetType(), cconn)
			case lSrc == nil && rSrc != nil && lDst != nil && rDst == nil:
				logging.GetLogger().Debugf("NSM: Got remote to local CrossConnect with id %s", cconnStr)
				if !isDelMsg && (rSrc.GetState() == remoteconn.State_DOWN || lDst.GetState() == localconn.State_DOWN) {
					logging.GetLogger().Debugf("NSM: one connection of the cross connect %s is in DOWN state, don't affect the skydive connections", cconn.GetId())
					continue
				}
				p.onConnRemoteLocal(event.GetType(), cconn)
			case lSrc != nil && rSrc == nil && lDst == nil && rDst != nil:
				logging.GetLogger().Debugf("NSM: Got local to remote CrossConnect with id %s", cconnStr)
				if !isDelMsg && (lSrc.GetState() == localconn.State_DOWN || rDst.GetState() == remoteconn.State_DOWN) {
					logging.GetLogger().Debugf("NSM: one connection of the cross connect %s is in DOWN state, don't affect the skydive connections", cconn.GetId())
					continue
				}
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
		logging.GetLogger().Debugf("NSM: adding local to local connection %+v", l)
		p.connections = append(p.connections, l)
		logging.GetLogger().Debugf("NSM: locking the graph for adding an Edge")
		p.g.Lock()
		l.addEdge(p.g)
		p.g.Unlock()
	} else {
		for i := range p.connections {
			l, ok := p.connections[i].(*localConnectionPair)
			if !ok {
				continue
			}
			if l.srcInode == srcInode && l.dstInode == dstInode {
				logging.GetLogger().Debugf("NSM: removing local to local connection %+v", l)
				logging.GetLogger().Debugf("NSM: locking the graph for deleting an Edge")
				p.g.Lock()
				l.delEdge(p.g)
				p.g.Unlock()
				p.removeConnectionItem(i)
				logging.GetLogger().Debugf("NSM: %v connection left", len(p.connections))
				break
			}
		}
	}
}

func (p *Probe) onConnLocalRemote(t cc.CrossConnectEventType, conn *cc.CrossConnect) {
	p.Lock()
	defer p.Unlock()
	c, i := p.getConnectionWithRemote(conn.GetRemoteDestination().GetId())
	if t != cc.CrossConnectEventType_DELETE {
		if c == nil {
			c = &remoteConnectionPair{
				baseConnectionPair: baseConnectionPair{
					src:     conn.GetLocalSource(),
					payload: conn.GetPayload(),
				},
				remote: conn.GetRemoteDestination(),
				srcID:  conn.GetId(),
				//TODO: compare payloads, they should be identical
			}
			logging.GetLogger().Debugf("NSM: adding local to remote connection %+v", c)
			p.connections = append(p.connections, c)
			//TODO: we erase previous payload if exist
			c.payload = conn.GetPayload()

		} else {
			logging.GetLogger().Debugf("NSM: adding source to local to remote connection %+v", c)
			c.src = conn.GetLocalSource()
			c.srcID = conn.GetId()
			//TODO: we erase previous payload if exist
			c.payload = conn.GetPayload()
			logging.GetLogger().Debugf("NSM: locking the graph for adding an Edge")
			p.g.Lock()
			c.addEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if c == nil {
			logging.GetLogger().Warning("NSM: received cross connect delete event for a connection that does not exist")
			return
		}

		logging.GetLogger().Debugf("NSM: locking the graph for deleting an Edge")
		p.g.Lock()
		c.delEdge(p.g)
		p.g.Unlock()

		// Delete the link only if dst is also empty
		if c.dst == nil {
			logging.GetLogger().Debugf("NSM: deleting local to remote connection %+v", c)
			p.removeConnectionItem(i)
			logging.GetLogger().Debugf("NSM: %v connection left", len(p.connections))
		} else {
			logging.GetLogger().Debugf("NSM: removing source from local to remote connection %+v", c)
			c.src = nil
		}
	}
}

func (p *Probe) onConnRemoteLocal(t cc.CrossConnectEventType, conn *cc.CrossConnect) {
	p.Lock()
	defer p.Unlock()
	c, i := p.getConnectionWithRemote(conn.GetRemoteSource().GetId())
	if t != cc.CrossConnectEventType_DELETE {
		if c == nil {
			c = &remoteConnectionPair{
				baseConnectionPair: baseConnectionPair{
					dst:     conn.GetLocalDestination(),
					payload: conn.GetPayload(),
				},
				remote: conn.GetRemoteSource(),
				dstID:  conn.GetId(),
				//TODO: compare payloads, they should be identical
			}
			logging.GetLogger().Debugf("NSM: adding remote to local connection %+v", c)
			p.connections = append(p.connections, c)
		} else {
			logging.GetLogger().Debugf("NSM: adding destination to remote to local connection %+v", c)
			c.dst = conn.GetLocalDestination()
			c.dstID = conn.GetId()
			//TODO: compare payloads, they should be identical
			c.payload = conn.GetPayload()
			logging.GetLogger().Debugf("NSM: locking the graph for adding an Edge")
			p.g.Lock()
			c.addEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if c == nil {
			logging.GetLogger().Warning("NSM: received cross connect delete event for a connection that does not exist")
			return
		}

		logging.GetLogger().Debugf("NSM: locking the graph for deleting an Edge")
		p.g.Lock()
		c.delEdge(p.g)
		p.g.Unlock()

		// Delete the link only if src is also empty
		if c.getSource() == nil {
			logging.GetLogger().Debugf("NSM: deleting local to remote connection %+v", c)
			p.removeConnectionItem(i)
			logging.GetLogger().Debugf("NSM: %v connection left", len(p.connections))
		} else {
			logging.GetLogger().Debugf("NSM: removing destination from remote to local connection %+v", c)
			c.dst = nil
		}
	}
}

// OnNodeAdded tell this probe a node in the graph has been added
func (p *Probe) OnNodeAdded(n *graph.Node) {
	if i, err := n.GetFieldInt64("Inode"); err == nil {
		// Find connections with matching inode
		logging.GetLogger().Debugf("NSM: node added with inode: %v", i)
		p.Lock()
		defer p.Unlock()
		c, err := p.getConnectionsWithInode(i)
		if err != nil {
			logging.GetLogger().Errorf("NSM: error retreiving connections with inodes : %v", err)
			return
		}
		if len(c) == 0 {
			logging.GetLogger().Debugf("NSM: no connection ready with inode %v", i)
			return
		}

		for i := range c {
			c[i].addEdge(p.g)
		}
	}
}

// OnNodeDeleted tell this probe a node in the graph has been deleted
func (p *Probe) OnNodeDeleted(n *graph.Node) {
	if i, err := n.GetFieldInt64("Inode"); err == nil {
		logging.GetLogger().Infof("NSM: node deleted with inode %v", i)
	}
	// If a graph node has been deleted, skydive should have automatically deleted egdes that was having this node as source or dest
	// TODO: Consider removing the corresponding connection from the connection list,
	// but it should be done by the corresponding CrossConnect DELETE events received from nsmds
}

// getConnectionsWithInode returns every connection that involves the inode, with a non empty source and/or destinatio
func (p *Probe) getConnectionsWithInode(inode int64) ([]connection, error) {
	var c []connection
	for i := range p.connections {
		src := p.connections[i].getSource()
		dst := p.connections[i].getDest()
		if src == nil || dst == nil {
			logging.GetLogger().Debugf("NSM: nil source or destination for connection %v", p.connections[i])
			continue
		}
		srcInode, err := getLocalInode(p.connections[i].getSource())
		if err != nil {
			return nil, err
		}
		dstInode, err := getLocalInode(p.connections[i].getDest())
		if err != nil {
			return nil, err
		}

		if srcInode == inode || dstInode == inode {
			c = append(c, p.connections[i])
		}
	}
	return c, nil
}

func (p *Probe) getConnectionWithRemote(id string) (*remoteConnectionPair, int) {
	// the probe has to be locked before calling this function
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

func (p *Probe) removeConnectionItem(i int) {
	// the probe has to be locked before calling this function

	// move last elem to position i and truncate the slice
	p.connections[i] = p.connections[len(p.connections)-1]
	p.connections[len(p.connections)-1] = nil
	p.connections = p.connections[:len(p.connections)-1]
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
	inodeStr, ok := conn.Mechanism.Parameters[localconn.NetNsInodeKey]
	if !ok {
		err := errors.New("NSM: no inodes in the connection parameters")
		logging.GetLogger().Error(err)
		return 0, err
	}
	inode, err := strconv.ParseInt(inodeStr, 10, 64)
	if err != nil {
		logging.GetLogger().Errorf("NSM: error converting inode %s to int64", inodeStr)
		return 0, err
	}
	return inode, nil
}
