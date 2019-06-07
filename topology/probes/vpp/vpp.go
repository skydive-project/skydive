// +build vpp,linux

//go:generate ${GOPATH}/.skydive-build-tool/binapi-generator --input-file=/usr/share/vpp/api/interface.api.json --output-dir=./bin_api
//go:generate ${GOPATH}/.skydive-build-tool/binapi-generator --input-file=/usr/share/vpp/api/vpe.api.json --output-dir=./bin_api

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
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.fd.io/govpp.git"
	"git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/core"

	"github.com/sirupsen/logrus"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/interfaces"
	"github.com/skydive-project/skydive/topology/probes/vpp/bin_api/vpe"
)

const (
	// VPPPollingTime in milliseconds
	VPPPollingTime = 200
)

// Probe is VPP probe
type Probe struct {
	sync.Mutex
	graph        *graph.Graph                              // the graph
	shm          string                                    // connect SHM path
	conn         *core.Connection                          // VPP connection
	interfaceMap map[uint32]*interfaces.SwInterfaceDetails // MAP of VPP interfaces
	root         *graph.Node                               // root node for ownership

	notifChan chan api.Message // notification channel on interfaces events
	state     int64            // state of the probe (running or stopped)
	wg        sync.WaitGroup   // goroutines wait group
}

func interfaceMAC(mac []byte) string {
	s := ""
	for i, m := range mac {
		if i != 0 {
			s += ":"
		}
		s += fmt.Sprintf("%02x", m)
	}
	return s
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
		logging.GetLogger().Error(err)
		return -1
	}
	return int64(msg.VrfID)
}

func (p *Probe) getInterface(index uint32) *graph.Node {
	p.graph.RLock()
	defer p.graph.RUnlock()
	return p.graph.LookupFirstNode(graph.Metadata{"IfIndex": int64(index), "Type": "vpp"})
}

func (p *Probe) createOrUpdateInterface(ch api.Channel, intf *interfaces.SwInterfaceDetails) *graph.Node {
	metadata := graph.Metadata{"IfIndex": int64(intf.SwIfIndex), "Type": "vpp"}

	g := p.graph
	g.Lock()
	defer g.Unlock()

	var err error
	node := p.graph.LookupFirstNode(metadata)
	if node == nil {
		node, err = g.NewNode(graph.GenID(), metadata)
		if err != nil {
			logging.GetLogger().Error(err)
			return nil
		}
		if _, err = g.Link(p.root, node, topology.OwnershipMetadata()); err != nil {
			logging.GetLogger().Error(err)
			return nil
		}
	}

	tr := g.StartMetadataTransaction(node)
	defer tr.Commit()
	tr.AddMetadata("Driver", "vpp")
	tr.AddMetadata("Name", strings.Trim(string(intf.InterfaceName), "\000"))
	tr.AddMetadata("IfIndex", int64(intf.SwIfIndex))
	tr.AddMetadata("MAC", interfaceMAC(intf.L2Address[:intf.L2AddressLength]))
	tr.AddMetadata("MTU", int64(intf.LinkMtu))
	tr.AddMetadata("Speed", int64(intf.LinkSpeed))
	state := interfaceUpDown(intf.AdminUpDown)
	tr.AddMetadata("State", state)
	if state != "DOWN" {
		tr.AddMetadata("Duplex", interfaceDuplex(intf.LinkDuplex))
	}
	tr.AddMetadata("VrfID", p.getInterfaceVrfID(ch, intf.SwIfIndex))

	return node
}

func interfaceNeedUpdate(i1, i2 *interfaces.SwInterfaceDetails) bool {
	if (i1.LinkMtu != i2.LinkMtu) ||
		(i1.LinkSpeed != i2.LinkSpeed) ||
		(i1.LinkDuplex != i2.LinkDuplex) {
		return true
	}
	return false
}

func (p *Probe) eventAddInterface(ch api.Channel, intf *interfaces.SwInterfaceDetails) {
	p.createOrUpdateInterface(ch, intf)
}

func (p *Probe) eventDelInterface(node *graph.Node) {
	p.graph.Lock()
	defer p.graph.Unlock()
	if err := p.graph.DelNode(node); err != nil {
		logging.GetLogger().Error(err)
	}
}

func (p *Probe) interfaceEventsEnableDisable(ch api.Channel, enable bool) {
	ed := uint32(0)
	if enable {
		ed = uint32(1)
	}
	req := &interfaces.WantInterfaceEvents{
		EnableDisable: ed,
		PID:           uint32(os.Getpid()),
	}
	msg := &interfaces.WantInterfaceEventsReply{}
	err := ch.SendRequest(req).ReceiveReply(msg)
	if err != nil {
		logging.GetLogger().Error(err)
	}
}

func (p *Probe) interfacesEvents() {
	defer p.wg.Done()

	ch, err := p.conn.NewAPIChannel()
	if err != nil {
		logging.GetLogger().Error("API channel error: ", err)
		return
	}

	sub, err := ch.SubscribeNotification(p.notifChan, &interfaces.SwInterfaceEvent{})
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	p.interfaceEventsEnableDisable(ch, true)

	for atomic.LoadInt64(&p.state) == common.RunningState {
		notif := <-p.notifChan
		if notif == nil {
			break
		}
		msg := notif.(*interfaces.SwInterfaceEvent)

		node := p.getInterface(msg.SwIfIndex)
		p.graph.RLock()
		name, _ := node.GetFieldString("Name")
		p.graph.RUnlock()
		if msg.Deleted > 0 {
			logging.GetLogger().Debugf("Delete interface %v idx %d", name, msg.SwIfIndex)
			p.eventDelInterface(node)
			continue
		}
		logging.GetLogger().Debugf("ChangeState interface %v idx %d updown %d", name, msg.SwIfIndex, msg.AdminUpDown)
		p.graph.Lock()
		node.Metadata.SetField("State", interfaceUpDown(msg.AdminUpDown))
		p.graph.Unlock()
	}

	p.interfaceEventsEnableDisable(ch, false)

	sub.Unsubscribe()
	ch.Close()
}

func (p *Probe) interfacesPolling() {
	defer p.wg.Done()

	ch, err := p.conn.NewAPIChannel()
	if err != nil {
		logging.GetLogger().Error("API channel error: ", err)
		return
	}

	for atomic.LoadInt64(&p.state) == common.RunningState {
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
				logging.GetLogger().Error(err)
				goto nextEvents
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
			logging.GetLogger().Debugf("Add/Update interface %v idx %d up/down %d", strings.Trim(string(msg.InterfaceName), "\000"), int64(msg.SwIfIndex), int64(msg.AdminUpDown))
			p.eventAddInterface(ch, msg)
		}
		/* Remove interface that didn't exist anymore */
		for index := range p.interfaceMap {
			_, found := foundInterfaces[index]
			_, firsttime := needUpdate[index]
			if !found && !firsttime {
				node := p.getInterface(index)
				p.graph.RLock()
				name, _ := node.GetFieldString("Name")
				p.graph.RUnlock()
				logging.GetLogger().Debugf("Delete interface %v idx %d", name, index)
				p.eventDelInterface(node)
				delete(p.interfaceMap, index)
			}
		}

	nextEvents:
		time.Sleep(VPPPollingTime * time.Millisecond)
	}

	ch.Close()
}

// Start VPP probe and get all interfaces
func (p *Probe) Start() {
	conn, err := govpp.Connect(p.shm)
	if err != nil {
		logging.GetLogger().Error("VPP connection error: ", err)
		return
	}
	p.conn = conn

	ch, err := conn.NewAPIChannel()
	if err != nil {
		logging.GetLogger().Error("API channel error: ", err)
		return
	}

	req := &vpe.ShowVersion{}
	msg := &vpe.ShowVersionReply{}
	err = ch.SendRequest(req).ReceiveReply(msg)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	ch.Close()

	metadata := graph.Metadata{
		"Name":      "vpp",
		"Type":      "vpp",
		"Program":   string(msg.Program),
		"Version":   string(msg.Version),
		"BuildDate": string(msg.BuildDate),
	}

	p.graph.Lock()
	defer p.graph.Unlock()
	n, err := p.graph.NewNode(graph.GenID(), metadata)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	topology.AddOwnershipLink(p.graph, p.root, n, nil)
	p.root = n

	atomic.StoreInt64(&p.state, common.RunningState)

	p.wg.Add(2)
	go p.interfacesPolling()
	go p.interfacesEvents()
}

// Stop the probe
func (p *Probe) Stop() {
	atomic.StoreInt64(&p.state, common.StoppingState)
	close(p.notifChan)
	p.conn.Disconnect()
	p.wg.Wait()
}

// NewProbe creates a VPP topology probe
func NewProbe(g *graph.Graph, shm string, root *graph.Node) *Probe {
	probe := &Probe{
		graph:        g,
		interfaceMap: make(map[uint32]*interfaces.SwInterfaceDetails),
		shm:          shm,
		root:         root,
		state:        common.StoppedState,
	}
	probe.notifChan = make(chan api.Message, 100)

	/* Forward all govpp logging to Skdyive logging */
	l := logrus.New()
	l.Out = ioutil.Discard
	l.Hooks.Add(probe)
	core.SetLogger(l)

	return probe
}

// NewProbeFromConfig initializes the probe
func NewProbeFromConfig(g *graph.Graph, root *graph.Node) (*Probe, error) {
	shm := config.GetString("agent.topology.vpp.connect")
	return NewProbe(g, shm, root), nil
}

// Levels Logrus to Skydive logger helper
func (p *Probe) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire Logrus to Skydive logger helper
func (p *Probe) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	case logrus.TraceLevel:
		logging.GetLogger().Debug(entry.String())
	case logrus.DebugLevel:
		logging.GetLogger().Debug(entry.String())
	case logrus.InfoLevel:
		logging.GetLogger().Info(entry.String())
	case logrus.WarnLevel:
		logging.GetLogger().Warning(entry.String())
	case logrus.ErrorLevel:
		logging.GetLogger().Error(entry.String())
	case logrus.FatalLevel:
		logging.GetLogger().Fatal(entry.String())
	case logrus.PanicLevel:
		logging.GetLogger().Panic(entry.String())
	}
	return nil
}
