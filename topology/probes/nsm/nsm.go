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
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/safchain/insanelock"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	cc "github.com/networkservicemesh/networkservicemesh/controlplane/api/crossconnect"
	localconn "github.com/networkservicemesh/networkservicemesh/controlplane/api/local/connection"
	remoteconn "github.com/networkservicemesh/networkservicemesh/controlplane/api/remote/connection"
	v1 "github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis/networkservice/v1alpha1"
	"github.com/networkservicemesh/networkservicemesh/k8s/pkg/networkservice/clientset/versioned"
	"github.com/networkservicemesh/networkservicemesh/k8s/pkg/networkservice/informers/externalversions"
	"google.golang.org/grpc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/logging"
)

const nsmResource = "networkservicemanagers"

var informerStopper chan struct{}

// Probe represents the NSM probe
type Probe struct {
	insanelock.RWMutex
	graph.DefaultGraphListener
	g     *graph.Graph
	state service.State
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
	probe.state.Store(service.StoppedState)
	return probe, nil
}

func getK8SConfig() (*rest.Config, error) {
	// check if CRD is installed
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	logging.GetLogger().Debugf("Unable to get in K8S cluster config, trying with KUBECONFIG env")

	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	return kubeconfig.ClientConfig()

}

// Start ...
func (p *Probe) Start() error {
	p.g.AddEventListener(p)
	p.state.Store(service.RunningState)

	config, err := getK8SConfig()
	if err != nil {
		return err
	}

	logging.GetLogger().Debug("NSM: getting NSM client")
	// Initialize clientset
	nsmClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Unable to initialize the NSM probe: %v", err)
	}

	factory := externalversions.NewSharedInformerFactory(nsmClientSet, 0)
	genericInformer, err := factory.ForResource(v1.SchemeGroupVersion.WithResource(nsmResource))
	if err != nil {
		return fmt.Errorf("Unable to create the K8S cache factory: %v", err)
	}

	informer := genericInformer.Informer()
	informerStopper := make(chan struct{})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			nsm := obj.(*v1.NetworkServiceManager)
			logging.GetLogger().Infof("New NSMgr Added: %v", nsm)
			go p.monitorCrossConnects(nsm.Spec.URL)
		},
	})

	go informer.Run(informerStopper)
	return nil
}

// Stop ....
func (p *Probe) Stop() {
	p.Lock()
	defer p.Unlock()
	if !p.state.CompareAndSwap(service.RunningState, service.StoppingState) {
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
		t := proto.TextMarshaler{}

		logging.GetLogger().Debugf("NSM: received monitoring event of type %s from %s with %v crossconnects", event.GetType(), url, len(event.GetCrossConnects()))
		for _, cconn := range event.GetCrossConnects() {
			cconnStr := t.Text(cconn)

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
				logging.GetLogger().Debugf("NSM: Got local to local CrossConnect:  %s", cconnStr)
				if !isDelMsg && (lSrc.GetState() == localconn.State_DOWN || lDst.GetState() == localconn.State_DOWN) {
					logging.GetLogger().Debugf("NSM: one connection of the cross connect %s is in DOWN state, don't affect the skydive connections", cconn.GetId())
					continue
				}
				p.onConnLocalLocal(event.GetType(), cconn, url)
			case lSrc == nil && rSrc != nil && lDst != nil && rDst == nil:
				logging.GetLogger().Debugf("NSM: Got remote to local CrossConnect: %s", cconnStr)
				if !isDelMsg && (rSrc.GetState() == remoteconn.State_DOWN || lDst.GetState() == localconn.State_DOWN) {
					logging.GetLogger().Debugf("NSM: one connection of the cross connect %s is in DOWN state, don't affect the skydive connections", cconn.GetId())
					continue
				}
				if rSrc.GetId() == "-" {
					logging.GetLogger().Warningf("NSM: received a crossconnect with remote id '-', filtering out this message")
					continue
				}
				p.onConnRemoteLocal(event.GetType(), cconn, url)
			case lSrc != nil && rSrc == nil && lDst == nil && rDst != nil:
				logging.GetLogger().Debugf("NSM: Got local to remote CrossConnect:  %s", cconnStr)
				if !isDelMsg && (lSrc.GetState() == localconn.State_DOWN || rDst.GetState() == remoteconn.State_DOWN) {
					logging.GetLogger().Debugf("NSM: one connection of the cross connect %s is in DOWN state, don't affect the skydive connections", cconn.GetId())
					continue
				}
				if rDst.GetId() == "-" {
					logging.GetLogger().Warningf("NSM: received a crossconnect with remote id '-', filtering out this message")
					continue
				}
				p.onConnLocalRemote(event.GetType(), cconn, url)
			default:
				logging.GetLogger().Errorf("NSM: Error parsing CrossConnect \n%s", cconnStr)
			}
		}
	}
}

func (p *Probe) onConnLocalLocal(t cc.CrossConnectEventType, xconn *cc.CrossConnect, url string) {

	p.Lock()
	defer p.Unlock()
	conn, i := p.getConnection(url, xconn.GetId())
	if t != cc.CrossConnectEventType_DELETE {
		if conn == nil {
			l := &localConnectionPair{
				cc: &crossConnect{
					ID:  xconn.GetId(),
					url: url,
				},
				baseConnectionPair: baseConnectionPair{
					payload: xconn.GetPayload(),
					src:     xconn.GetLocalSource(),
					dst:     xconn.GetLocalDestination(),
				},
			}
			p.addConnection(l)
			p.g.Lock()
			l.addEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if conn != nil {
			p.removeConnectionItem(i)
			p.g.Lock()
			conn.delEdge(p.g)
			p.g.Unlock()
		} else {
			logging.GetLogger().Warningf("NSM: received a DELETE from %s for crossconnect id %s, but the connection does not exists in the probe", url, xconn.GetId())
		}
	}
}

func (p *Probe) onConnLocalRemote(t cc.CrossConnectEventType, conn *cc.CrossConnect, url string) {
	p.Lock()
	defer p.Unlock()
	c, i := p.getConnectionWithRemote(conn.GetRemoteDestination())
	if t != cc.CrossConnectEventType_DELETE {
		if c == nil {
			c = &remoteConnectionPair{
				baseConnectionPair: baseConnectionPair{
					src:     conn.GetLocalSource(),
					payload: conn.GetPayload(),
				},
				remote: conn.GetRemoteDestination(),
				srcCc: &crossConnect{
					ID:  conn.GetId(),
					url: url,
				},
			}
			p.addConnection(c)

		} else {
			c.src = conn.GetLocalSource()
			c.srcCc = &crossConnect{
				ID:  conn.GetId(),
				url: url,
			}
			//TODO: compare payloads, they should be identical
			c.payload = conn.GetPayload()
			logging.GetLogger().Infof("NSM: local source added to remote connection %s", c.printCrossConnect())
			p.g.Lock()
			c.addEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if c == nil {
			logging.GetLogger().Warning("NSM: received cross connect delete event for a connection that does not exist")
			return
		}

		p.g.Lock()
		c.delEdge(p.g)
		p.g.Unlock()

		// Delete the link only if dst is also empty
		if c.dst == nil {
			p.removeConnectionItem(i)
		} else {
			logging.GetLogger().Infof("NSM: removing source from local to remote connection %+v", c)
			c.src = nil
			c.srcCc = nil
		}
	}
}

func (p *Probe) onConnRemoteLocal(t cc.CrossConnectEventType, conn *cc.CrossConnect, url string) {
	p.Lock()
	defer p.Unlock()
	c, i := p.getConnectionWithRemote(conn.GetRemoteSource())
	if t != cc.CrossConnectEventType_DELETE {
		if c == nil {
			c = &remoteConnectionPair{
				baseConnectionPair: baseConnectionPair{
					dst:     conn.GetLocalDestination(),
					payload: conn.GetPayload(),
				},
				remote: conn.GetRemoteSource(),
				dstCc: &crossConnect{
					ID:  conn.GetId(),
					url: url,
				},
			}
			p.addConnection(c)
		} else {
			c.dst = conn.GetLocalDestination()
			c.dstCc = &crossConnect{
				ID:  conn.GetId(),
				url: url,
			}
			//TODO: compare payloads, they should be identical
			c.payload = conn.GetPayload()
			logging.GetLogger().Infof("NSM: local destination added to remote connection %s", c.printCrossConnect())
			p.g.Lock()
			c.addEdge(p.g)
			p.g.Unlock()
		}
	} else {
		if c == nil {
			logging.GetLogger().Warning("NSM: received cross connect delete event for a connection that does not exist")
			return
		}

		p.g.Lock()
		c.delEdge(p.g)
		p.g.Unlock()

		// Delete the link only if src is also empty
		if c.getSource() == nil {
			p.removeConnectionItem(i)
		} else {
			logging.GetLogger().Infof("NSM: removing destination from remote to local connection %+v", c)
			c.dst = nil
			c.dstCc = nil
		}
	}
}

// OnNodeAdded tell this probe a node in the graph has been added
func (p *Probe) OnNodeAdded(n *graph.Node) {
	if inode, err := n.GetFieldInt64("Inode"); err == nil {
		// Find connections with matching inode
		p.Lock()
		defer p.Unlock()
		c, err := p.getConnectionsReadyWithInode(inode)
		if err != nil {
			logging.GetLogger().Errorf("NSM: error retreiving connections with inodes : %v", err)
			return
		}
		if len(c) == 0 {
			return
		}

		for i := range c {
			c[i].addEdge(p.g)
		}
	}
}

// If a graph node has been deleted, skydive should have automatically deleted egdes that was having this node as source or dest
// connection removal from the connection list should be done by the corresponding CrossConnect DELETE events received from nsmds

// getConnectionsReadyWithInode returns every connection that involves the inode, with a non empty source and/or destination
func (p *Probe) getConnectionsReadyWithInode(inode int64) ([]connection, error) {
	var c []connection
	for i := range p.connections {
		srcInode, dstInode := p.connections[i].getInodes()
		if srcInode == 0 || dstInode == 0 {
			logging.GetLogger().Debugf("NSM: nil source or destination for connection, %s", p.connections[i].printCrossConnect())
			continue
		}

		if srcInode == inode || dstInode == inode {
			c = append(c, p.connections[i])
		}
	}
	return c, nil
}

func (p *Probe) getConnection(url string, id string) (connection, int) {
	for i := range p.connections {

		if p.connections[i].isCrossConnectOwner(url, id) {
			return p.connections[i], i
		}
	}
	return nil, 0
}

func (p *Probe) getConnectionWithRemote(remote *remoteconn.Connection) (*remoteConnectionPair, int) {
	// the probe has to be locked before calling this function

	// since the remote crossconnect id is issued by the responder to the connection request,
	//which is the DestinationNetworkServiceManagerName,
	// it is unique only in the context of this nsmgr
	// Then to check two remote connections are identical, we check the id and the destination nsmgr
	for i := range p.connections {
		c, ok := p.connections[i].(*remoteConnectionPair)
		if !ok {
			continue
		}

		if c.remote.GetId() == remote.GetId() && c.remote.GetDestinationNetworkServiceManagerName() == remote.GetDestinationNetworkServiceManagerName() {
			return c, i
		}
	}
	return nil, 0
}

func (p *Probe) addConnection(c connection) {
	logging.GetLogger().Infof("NSM: adding connection, %+s", c.printCrossConnect())
	p.connections = append(p.connections, c)
	logging.GetLogger().Infof("NSM: connections count: %v", len(p.connections))
}

func (p *Probe) removeConnectionItem(i int) {
	// the probe has to be locked before calling this function

	logging.GetLogger().Infof("NSM: deleting connection, %s", p.connections[i].printCrossConnect())
	// move last elem to position i and truncate the slice
	p.connections[i] = p.connections[len(p.connections)-1]
	p.connections[len(p.connections)-1] = nil
	p.connections = p.connections[:len(p.connections)-1]
	logging.GetLogger().Infof("NSM: %v connections left", len(p.connections))
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

// Register registers graph metadata decoders
func Register() {
	graph.EdgeMetadataDecoders["NSM"] = MetadataDecoder
}
