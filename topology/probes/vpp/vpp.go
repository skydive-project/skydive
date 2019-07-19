// +build vpp,linux

//go:generate go run git.fd.io/govpp.git/cmd/binapi-generator --input-file=/usr/share/vpp/api/interface.api.json --output-dir=./bin_api
//go:generate go run git.fd.io/govpp.git/cmd/binapi-generator --input-file=/usr/share/vpp/api/vpe.api.json --output-dir=./bin_api

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
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	govpp "git.fd.io/govpp.git"
	"git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/core"

	"github.com/sirupsen/logrus"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
	tp "github.com/skydive-project/skydive/topology/probes"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/interfaces"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/vpe"
)

const (
	// VPPPollingTime in milliseconds
	VPPPollingTime          = 10 * time.Second
	HealthCheckReplyTimeout = 3 * time.Second
	HealthCheckInterval     = 3 * time.Second
)

// Probe is an instance of a VPP probe in a namespace
type Probe struct {
	sync.Mutex
	*graph.EventHandler
	Ctx          tp.Context
	addr         string                                    // VPP address (unix or SHM)
	conn         *core.Connection                          // VPP connection
	interfaceMap map[uint32]*interfaces.SwInterfaceDetails // map of VPP interfaces
	intfIndexer  *graph.MetadataIndexer                    // index of created nodes by the probe
	vppRootNode  *graph.Node                               // root node for ownership
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

func (p *Probe) createOrUpdateInterface(ch api.Channel, intf *interfaces.SwInterfaceDetails) *graph.Node {
	vrfID := p.getInterfaceVrfID(ch, intf.SwIfIndex)

	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	var err error
	node := p.getInterface(intf.SwIfIndex)
	if node == nil {
		node, err = p.Ctx.Graph.NewNode(graph.GenID(), graph.Metadata{"IfIndex": int64(intf.SwIfIndex), "Type": "vpp"})
		if err != nil {
			p.Ctx.Logger.Error(err)
			return nil
		}
		p.NotifyEvent(graph.NodeAdded, node)

		if _, err = p.Ctx.Graph.Link(p.vppRootNode, node, topology.OwnershipMetadata()); err != nil {
			p.Ctx.Logger.Error(err)
			return nil
		}
	}

	tr := p.Ctx.Graph.StartMetadataTransaction(node)
	tr.AddMetadata("Driver", "vpp")
	tr.AddMetadata("Name", interfaceName(intf.InterfaceName))
	tr.AddMetadata("IfIndex", int64(intf.SwIfIndex))
	tr.AddMetadata("MAC", interfaceMAC(intf.L2Address[:intf.L2AddressLength]))
	tr.AddMetadata("MTU", int64(intf.LinkMtu))
	tr.AddMetadata("Speed", int64(intf.LinkSpeed))
	state := interfaceUpDown(intf.AdminUpDown)
	tr.AddMetadata("State", state)
	if state != "DOWN" {
		tr.AddMetadata("Duplex", interfaceDuplex(intf.LinkDuplex))
	}
	tr.AddMetadata("VrfID", vrfID)
	tr.Commit()

	p.NotifyEvent(graph.NodeUpdated, node)
	return node
}

func interfaceNeedUpdate(i1, i2 *interfaces.SwInterfaceDetails) bool {
	return i1.LinkMtu != i2.LinkMtu ||
		i1.LinkSpeed != i2.LinkSpeed ||
		i1.LinkDuplex != i2.LinkDuplex
}

func (p *Probe) eventAddInterface(ch api.Channel, intf *interfaces.SwInterfaceDetails) {
	p.createOrUpdateInterface(ch, intf)
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
		}
		if err != nil {
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
	for index := range needUpdate {
		msg := p.interfaceMap[index]
		p.Ctx.Logger.Debugf("Add/Update interface %s idx %d up/down %s", interfaceName(msg.InterfaceName), int64(msg.SwIfIndex), interfaceUpDown(msg.AdminUpDown))
		p.eventAddInterface(ch, msg)
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

	notifChan := make(chan api.Message, 100)
	sub, err := ch.SubscribeNotification(notifChan, &interfaces.SwInterfaceEvent{})
	if err != nil {
		p.Ctx.Logger.Error(err)
		return
	}

	p.Ctx.Logger.Debugf("Registering for VPP events")
	p.interfaceEventsEnableDisable(ch, true)

	pollingTicker := time.NewTicker(VPPPollingTime)
	defer pollingTicker.Stop()

	if err := p.synchronize(ch); err != nil {
		p.Ctx.Logger.Error(err)
		return
	}

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

		p.intfIndexer.Start()
		defer p.intfIndexer.Stop()

		p.Ctx.Graph.RLock()
		p.intfIndexer.Sync()
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
	}

	p.intfIndexer = graph.NewMetadataIndexer(p.Ctx.Graph, p, nil, "IfIndex")

	/* Forward all govpp logging to Skydive logging */
	l := logrus.New()
	l.Out = ioutil.Discard
	l.Hooks.Add(p)
	core.SetLogger(l)

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
