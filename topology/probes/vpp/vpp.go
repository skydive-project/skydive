// +build vpp,linux

//go:generate go run git.fd.io/govpp.git/cmd/binapi-generator --input-file=/usr/share/vpp/api/interface.api.json --output-dir=./bin_api
//go:generate go run git.fd.io/govpp.git/cmd/binapi-generator --input-file=/usr/share/vpp/api/vpe.api.json --output-dir=./bin_api
//go:generate go run git.fd.io/govpp.git/cmd/binapi-generator --input-file=/usr/share/vpp/api/memif.api.json --output-dir=./bin_api

/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package vpp

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/skydive-project/skydive/common"

	govpp "git.fd.io/govpp.git"
	"git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/core"

	"github.com/sirupsen/logrus"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
	tp "github.com/skydive-project/skydive/topology/probes"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/interfaces"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/memif"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/vpe"
)

const (
	// VPPPollingTime in milliseconds
	VPPPollingTime          = 10 * time.Second
	HealthCheckReplyTimeout = 3 * time.Second
	HealthCheckInterval     = 3 * time.Second
)

var memifModes = map[uint8]string{
	0: "ethernet",
	1: "ip",
	2: "punt/inject",
}

// Probe is an instance of a VPP probe in a namespace
type Probe struct {
	sync.Mutex
	*graph.EventHandler
	graph.DefaultGraphListener
	Ctx          tp.Context
	addr         string                                    // VPP address (unix or SHM)
	conn         *core.Connection                          // VPP connection
	interfaceMap map[uint32]*interfaces.SwInterfaceDetails // map of VPP interfaces
	intfIndexer  *graph.MetadataIndexer                    // index of created nodes by the probe
	vppRootNode  *graph.Node                               // root node for ownership
	socketsDirs  []string
	masterMemifs *graph.NodeIndex
	slaveMemifs  *graph.NodeIndex
	seeds        map[int64]*exec.Cmd
}

func interfaceName(name []byte) string {
	return strings.Trim(string(name), "\000")
}

func interfaceMAC(mac []byte) string {
	return net.HardwareAddr(mac).String()
}

func interfaceDuplex(duplex uint8) string {
	switch duplex {
	case 1:
		return "half"
	case 2:
		return "full"
	}
	return "unknown"
}

func interfaceUpDown(updown uint8) string {
	if updown == 1 {
		return "UP"
	}
	return "DOWN"
}

func (p *Probe) getInterfaceVrfID(ch api.Channel, index uint32) int64 {
	req := &interfaces.SwInterfaceGetTable{SwIfIndex: index}
	msg := &interfaces.SwInterfaceGetTableReply{}
	err := ch.SendRequest(req).ReceiveReply(msg)
	if err != nil {
		p.Ctx.Logger.Error(err)
		return -1
	}
	return int64(msg.VrfID)
}

func (p *Probe) getInterface(index uint32) *graph.Node {
	node, _ := p.intfIndexer.GetNode(int64(index))
	return node
}

func (p *Probe) createOrUpdateInterface(ch api.Channel, intf *interfaces.SwInterfaceDetails, metadata graph.Metadata) *graph.Node {
	vrfID := p.getInterfaceVrfID(ch, intf.SwIfIndex)
	name := interfaceName(intf.InterfaceName)

	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	metadata["Driver"] = "vpp"
	metadata["Type"] = "interface"
	metadata["Name"] = name
	metadata["IfIndex"] = int64(intf.SwIfIndex)
	metadata["MAC"] = interfaceMAC(intf.L2Address[:intf.L2AddressLength])
	metadata["MTU"] = int64(intf.LinkMtu)
	metadata["Speed"] = int64(intf.LinkSpeed)
	metadata["State"] = interfaceUpDown(intf.AdminUpDown)
	metadata["Duplex"] = interfaceDuplex(intf.LinkDuplex)
	metadata["VrfID"] = vrfID

	if strings.HasPrefix(name, "host-") {
		metadata["InterfaceName"] = name[5:]
	}

	var err error
	node := p.getInterface(intf.SwIfIndex)
	if node == nil {
		node, err = p.Ctx.Graph.NewNode(graph.GenID(), metadata)
		if err != nil {
			p.Ctx.Logger.Error(err)
			return nil
		}
		p.NotifyEvent(graph.NodeAdded, node)

		if _, err = p.Ctx.Graph.Link(p.vppRootNode, node, topology.OwnershipMetadata()); err != nil {
			p.Ctx.Logger.Error(err)
			return nil
		}
	} else {
		p.Ctx.Graph.SetMetadata(node, metadata)
		p.NotifyEvent(graph.NodeUpdated, node)
	}
	return node
}

func interfaceNeedUpdate(i1, i2 *interfaces.SwInterfaceDetails) bool {
	return i1.LinkMtu != i2.LinkMtu ||
		i1.LinkSpeed != i2.LinkSpeed ||
		i1.LinkDuplex != i2.LinkDuplex
}

func (p *Probe) eventAddInterface(ch api.Channel, intf *interfaces.SwInterfaceDetails, metadata graph.Metadata) *graph.Node {
	return p.createOrUpdateInterface(ch, intf, metadata)
}

func (p *Probe) eventDelInterface(node *graph.Node) {
	if err := p.Ctx.Graph.DelNode(node); err != nil {
		p.NotifyEvent(graph.NodeDeleted, node)
		p.Ctx.Logger.Error(err)
	}
}

func (p *Probe) interfaceEventsEnableDisable(ch api.Channel, enable bool) {
	req := &interfaces.WantInterfaceEvents{PID: uint32(os.Getpid())}
	if enable {
		req.EnableDisable = 1
	}
	msg := &interfaces.WantInterfaceEventsReply{}
	if err := ch.SendRequest(req).ReceiveReply(msg); err != nil {
		p.Ctx.Logger.Error(err)
	}
}

type memifSocket struct {
	*memif.MemifDetails
	filename string
}

func (p *Probe) fetchMemif(ch api.Channel) (map[uint32]*memifSocket, error) {
	sharedMemifSockets := make(map[uint32]*memifSocket)

	req := &memif.MemifSocketFilenameDump{}
	msg := &memif.MemifSocketFilenameDetails{}
	reqCtx := ch.SendMultiRequest(req)
	for {
		if lastReply, err := reqCtx.ReceiveReply(msg); lastReply {
			break
		} else if err != nil {
			return nil, err
		}

		sharedMemifSockets[msg.SocketID] = &memifSocket{filename: interfaceName(msg.SocketFilename)}
	}

	req2 := &memif.MemifDump{}
	msg2 := &memif.MemifDetails{}
	reqCtx = ch.SendMultiRequest(req2)
	for {
		if lastReply, err := reqCtx.ReceiveReply(msg2); lastReply {
			break
		} else if err != nil {
			return nil, err
		}

		sharedMemifSockets[msg2.SocketID].MemifDetails = msg2
	}

	return sharedMemifSockets, nil
}

func (p *Probe) synchronize(ch api.Channel) error {
	foundInterfaces := make(map[uint32]struct{})
	needUpdate := make(map[uint32]struct{})

	req := &interfaces.SwInterfaceDump{}
	reqCtx := ch.SendMultiRequest(req)
	for {
		msg := &interfaces.SwInterfaceDetails{}
		stop, err := reqCtx.ReceiveReply(msg)
		if stop {
			break
		} else if err != nil {
			return err
		}

		intf, found := p.interfaceMap[msg.SwIfIndex]
		if !found || interfaceNeedUpdate(msg, intf) {
			needUpdate[msg.SwIfIndex] = struct{}{}
		}
		if found {
			foundInterfaces[msg.SwIfIndex] = struct{}{}
		}
		p.interfaceMap[msg.SwIfIndex] = msg
	}

	/* Update interface metadata */
	var sharedMemifSockets map[uint32]*memifSocket
	for index := range needUpdate {
		metadata := make(graph.Metadata)
		msg := p.interfaceMap[index]
		ifName := interfaceName(msg.InterfaceName)
		p.Ctx.Logger.Debugf("Add/Update interface %s idx %d up/down %s", ifName, int64(msg.SwIfIndex), interfaceUpDown(msg.AdminUpDown))

		var socket, id int
		if _, err := fmt.Sscanf(string(ifName), "memif%d/%d", &socket, &id); err == nil {
			metadata["Type"] = "memif"

			if sharedMemifSockets == nil {
				if sharedMemifSockets, err = p.fetchMemif(ch); err != nil {
					return err
				}
			}

			if memifSock, ok := sharedMemifSockets[uint32(socket)]; ok {
				metadata["VPP"] = &Metadata{
					SocketFilename: memifSock.filename,
					ID:             int64(memifSock.ID),
					SocketID:       int64(socket),
					Master:         memifSock.Role == 0,
					Mode:           memifModes[memifSock.Mode],
					RingSize:       int64(memifSock.RingSize),
					BufferSize:     int64(memifSock.BufferSize),
					LinkUpDown:     memifSock.LinkUpDown == 1,
				}
			}
		}

		p.eventAddInterface(ch, msg, metadata)
	}

	/* Remove interface that didn't exist anymore */
	for index := range p.interfaceMap {
		_, found := foundInterfaces[index]
		_, firsttime := needUpdate[index]
		if !found && !firsttime {
			p.Ctx.Graph.Lock()
			if node := p.getInterface(index); node != nil {
				name, _ := node.GetFieldString("Name")
				p.Ctx.Logger.Debugf("Delete interface %v idx %d", name, index)
				p.eventDelInterface(node)
			}
			p.Ctx.Graph.Unlock()
			delete(p.interfaceMap, index)
		}
	}

	return nil
}

func (p *Probe) getHostSocketFilename(socketFilename string, n *graph.Node) (string, error) {
	vppNode := topology.GetOwner(p.Ctx.Graph, n)
	if vppNode == nil {
		return "", fmt.Errorf("No owner found for node %s", n.ID)
	}

	rootNode := topology.GetOwner(p.Ctx.Graph, vppNode)
	if rootNode == nil {
		return "", fmt.Errorf("No root owner found for node %s", vppNode.ID)
	}

	containerNode := p.Ctx.Graph.LookupFirstChild(rootNode, graph.Metadata{"Type": "container"})
	if containerNode == nil {
		return "", fmt.Errorf("No container found in node %s", rootNode.ID)
	}

	if dockerMetadata, err := containerNode.GetField("Docker"); err == nil {
		dockerMetadata := dockerMetadata.(*docker.Metadata)
		for _, mount := range dockerMetadata.Mounts {
			if filepath.HasPrefix(socketFilename, mount.Destination) {
				if hostPath, err := filepath.Rel(mount.Destination, socketFilename); err == nil {
					return filepath.Join(mount.Source, hostPath), nil
				}
			}
		}
	}

	return "", nil
}

func (p *Probe) indexSocketFilename(socketFilename string, intfNode *graph.Node) {
	socketFilename, err := p.getHostSocketFilename(socketFilename, intfNode)

	if err != nil {
		p.Ctx.Logger.Error(err)
		return
	}

	if socketFilename == "" {
		return
	}

	if isMaster, _ := intfNode.GetFieldBool("VPP.Master"); isMaster {
		p.masterMemifs.Index(intfNode.ID, intfNode, map[string]interface{}{socketFilename: nil})
	} else {
		p.slaveMemifs.Index(intfNode.ID, intfNode, map[string]interface{}{socketFilename: nil})
	}

	p.Ctx.Logger.Debugf("Indexed %s", socketFilename)
}

// Register a seed with the specified mount namespace
func (p *Probe) startSeed(pid int64, root *graph.Node) (*exec.Cmd, error) {
	if config.GetString("agent.auth.api.backend") != "noauth" {
		return nil, errors.New("Starting seeds is only supported with the 'noauth' authentication backend")
	}

	seed := os.Getenv("SKYDIVE_SEED_EXECUTABLE")
	if seed == "" {
		seed = os.Args[0]
	}
	nsPath := fmt.Sprintf("/proc/%d/ns", pid)
	sa, _ := common.ServiceAddressFromString(config.GetString("agent.listen"))
	command := exec.Command(seed, "seed",
		"--root="+string(root.ID),
		"--pid="+path.Join(nsPath, "pid"),
		"--mount="+path.Join(nsPath, "mnt"),
		"--agent="+fmt.Sprintf("%s:%d", sa.Addr, sa.Port),
		fmt.Sprintf("--filter=G.V('%s').Descendants().Subgraph()", root.ID),
		"vpp")
	command.Env = append(command.Env, fmt.Sprintf("SKYDIVE_LOGGING_LEVEL=%s", p.Ctx.Config.GetString("logging.level")))
	command.Env = append(command.Env, fmt.Sprintf("SKYDIVE_HOST_ID=vpp-%s", root.ID))
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	p.Ctx.Logger.Debugf("Starting seed in pid and mount namespaces of process %d (%s)", pid, strings.Join(command.Args, " "))

	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s", err)
	}

	go command.Wait()

	return command, nil
}

func (p *Probe) OnEdgeAdded(e *graph.Edge) {
	if edgeType, _ := e.GetFieldString("RelationType"); edgeType != topology.OwnershipLink {
		return
	}

	intfNode := p.Ctx.Graph.GetNode(e.Child)
	if s, _ := intfNode.GetFieldString("VPP.SocketFilename"); s != "" {
		p.indexSocketFilename(s, intfNode)
	} else if pid, err := intfNode.GetFieldInt64("InitProcessPID"); err == nil {
		parent := p.Ctx.Graph.GetNode(e.Parent)
		if t, _ := parent.GetFieldString("Type"); t == "netns" {
			if _, found := p.seeds[pid]; !found {
				seed, err := p.startSeed(pid, parent)
				if err != nil {
					p.Ctx.Logger.Errorf("Failed to start seed for %d: %s", pid, err)
					return
				}
				p.seeds[pid] = seed
			}
		}
	}
}

func (p *Probe) GetABLinks(node *graph.Node) (edges []*graph.Edge) {
	socketFilename, _ := node.GetFieldString("VPP.SocketFilename")
	socketFilename, _ = p.getHostSocketFilename(socketFilename, node)
	nodes, _ := p.slaveMemifs.Get(socketFilename)
	for _, slave := range nodes {
		edges = append(edges, topology.NewLink(p.Ctx.Graph, node, slave, "memif", nil))
	}
	return
}

func (p *Probe) GetBALinks(node *graph.Node) (edges []*graph.Edge) {
	socketFilename, _ := node.GetFieldString("VPP.SocketFilename")
	socketFilename, _ = p.getHostSocketFilename(socketFilename, node)
	nodes, _ := p.masterMemifs.Get(socketFilename)
	for _, master := range nodes {
		edges = append(edges, topology.NewLink(p.Ctx.Graph, master, node, "memif", nil))
	}
	return
}

func (p *Probe) handleInterfaceEvent(msg *interfaces.SwInterfaceEvent) {
	p.Ctx.Graph.Lock()
	if node := p.getInterface(msg.SwIfIndex); node != nil {
		name, _ := node.GetFieldString("Name")
		if msg.Deleted > 0 {
			p.Ctx.Logger.Debugf("Delete interface %v idx %d", name, msg.SwIfIndex)
			p.eventDelInterface(node)
		} else {
			state := interfaceUpDown(msg.AdminUpDown)
			p.Ctx.Logger.Debugf("ChangeState interface %s idx %d updown %d", name, msg.SwIfIndex, state)
			p.Ctx.Graph.AddMetadata(node, "State", state)
		}
	}
	p.Ctx.Graph.Unlock()
}

func (p *Probe) run(ctx context.Context) {
	ch, err := p.conn.NewAPIChannel()
	if err != nil {
		p.Ctx.Logger.Error("API channel error: ", err)
		return
	}
	ch.SetReplyTimeout(HealthCheckReplyTimeout)

	notifChan := make(chan api.Message, 10)
	sub, err := ch.SubscribeNotification(notifChan, &interfaces.SwInterfaceEvent{})
	if err != nil {
		p.Ctx.Logger.Error(err)
		return
	}

	p.Ctx.Logger.Debugf("Registering for VPP events")
	p.interfaceEventsEnableDisable(ch, true)

	if err := p.synchronize(ch); err != nil {
		p.Ctx.Logger.Error(err)
		return
	}

	pollingTicker := time.NewTicker(VPPPollingTime)
	defer pollingTicker.Stop()

	pingTimer := time.NewTimer(HealthCheckInterval)
	defer pingTimer.Stop()

LOOP:
	for {
		select {
		case notif := <-notifChan:
			msg := notif.(*interfaces.SwInterfaceEvent)
			p.Ctx.Logger.Debugf("Received sw interface event %+v", msg)
			p.handleInterfaceEvent(msg)
		case <-pollingTicker.C:
			err = p.synchronize(ch)
		case <-pingTimer.C:
			req := &vpe.ControlPing{}
			msg := &vpe.ControlPingReply{}
			if err = ch.SendRequest(req).ReceiveReply(msg); err != nil {
				err = fmt.Errorf("Health check failed: %s", err)
			}
		case <-ctx.Done():
			defer p.conn.Disconnect()
			break LOOP
		}

		if err != nil {
			p.Ctx.Logger.Error(err)
			break
		}

		pingTimer.Reset(HealthCheckInterval)
	}

	p.Ctx.Logger.Debugf("Unregistering for VPP events")
	p.interfaceEventsEnableDisable(ch, false)

	sub.Unsubscribe()
	ch.Close()
}

// Do starts the VPP probe and get all interfaces
func (p *Probe) Do(ctx context.Context, wg *sync.WaitGroup) error {
	conn, err := govpp.Connect(p.addr)
	if err != nil {
		return fmt.Errorf("VPP connection error: %s", err)
	}
	p.conn = conn

	if p.vppRootNode == nil {
		ch, err := conn.NewAPIChannel()
		if err != nil {
			return fmt.Errorf("API channel error: %s", err)
		}

		req := &vpe.ShowVersion{}
		msg := &vpe.ShowVersionReply{}
		err = ch.SendRequest(req).ReceiveReply(msg)
		if err != nil {
			return err
		}
		ch.Close()

		metadata := graph.Metadata{
			"Name":      "vpp",
			"Type":      "vpp",
			"Program":   string(msg.Program),
			"Version":   string(msg.Version),
			"BuildDate": string(msg.BuildDate),
		}

		p.Ctx.Graph.Lock()
		defer p.Ctx.Graph.Unlock()

		if p.vppRootNode, err = p.Ctx.Graph.NewNode(graph.GenID(), metadata); err != nil {
			return err
		}

		if _, err := topology.AddOwnershipLink(p.Ctx.Graph, p.Ctx.RootNode, p.vppRootNode, nil); err != nil {
			return err
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		hostNameIndexer := graph.NewMetadataIndexer(p.Ctx.Graph, p.Ctx.Graph, graph.Metadata{"Type": "veth"}, "Name")
		vppNameIndexer := graph.NewMetadataIndexer(p.Ctx.Graph, p, nil, "InterfaceName")
		nameLinker := graph.NewMetadataIndexerLinker(p.Ctx.Graph, hostNameIndexer, vppNameIndexer, topology.Layer2Metadata())

		p.intfIndexer.Start()
		defer p.intfIndexer.Stop()

		hostNameIndexer.Start()
		defer hostNameIndexer.Stop()

		vppNameIndexer.Start()
		defer vppNameIndexer.Stop()

		nameLinker.Start()
		defer nameLinker.Stop()

		p.Ctx.Graph.RLock()
		p.intfIndexer.Sync()
		hostNameIndexer.Sync()
		vppNameIndexer.Sync()
		p.Ctx.Graph.RUnlock()

		p.run(ctx)
	}()

	return nil
}

// NewProbe returns a new VPP probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	addr := ctx.Config.GetString("agent.topology.vpp.connect")

	p := &Probe{
		EventHandler: graph.NewEventHandler(100),
		Ctx:          ctx,
		addr:         addr,
		interfaceMap: make(map[uint32]*interfaces.SwInterfaceDetails),
		masterMemifs: graph.NewNodeIndex(ctx.Graph, false),
		slaveMemifs:  graph.NewNodeIndex(ctx.Graph, false),
		seeds:        make(map[int64]*exec.Cmd),
	}

	p.intfIndexer = graph.NewMetadataIndexer(p.Ctx.Graph, p, graph.Metadata{"Host": ctx.Graph.GetHost()}, "IfIndex")

	// This linkers creates edges between memif interfaces.
	// It is started in the Init method so that links are created by
	// the agent (not the seeds), even if VPP is not running on the host
	// (only in the containers)
	memifLinker := graph.NewResourceLinker(p.Ctx.Graph, []graph.ListenerHandler{p.masterMemifs}, []graph.ListenerHandler{p.slaveMemifs}, p, nil)
	memifLinker.Start()

	/* Forward all govpp logging to Skydive logging */
	l := logrus.New()
	l.Out = ioutil.Discard
	l.Hooks.Add(p)
	core.SetLogger(l)

	ctx.Graph.AddEventListener(p)

	return probes.NewProbeWrapper(p), nil
}

// Levels Logrus to Skydive logger helper
func (p *Probe) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire Logrus to Skydive logger helper
func (p *Probe) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	case logrus.TraceLevel:
		p.Ctx.Logger.Debug(entry.String())
	case logrus.DebugLevel:
		p.Ctx.Logger.Debug(entry.String())
	case logrus.InfoLevel:
		p.Ctx.Logger.Info(entry.String())
	case logrus.WarnLevel:
		p.Ctx.Logger.Warning(entry.String())
	case logrus.ErrorLevel:
		p.Ctx.Logger.Error(entry.String())
	case logrus.FatalLevel:
		p.Ctx.Logger.Fatal(entry.String())
	case logrus.PanicLevel:
		p.Ctx.Logger.Panic(entry.String())
	}
	return nil
}
