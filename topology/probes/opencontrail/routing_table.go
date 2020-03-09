// +build linux,opencontrail
//go:generate go run github.com/mailru/easyjson/easyjson -build_tags opencontrail $GOFILE

/*
 * Copyright (C) 2018 Orange, Inc.
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

// When an interface node is created, the VRFID is get from the
// Contrail Vrouter Agent and associated to this node. This VRF is
// then dumped (with rt --dump) to populate the Contrail.RoutingTable
// metadata.
//
// The process rt --monitor is spawn to get route update notifications
// from the Contrail vrouter kernel module. All route updates contain
// the VRFID. This VRFID is then used to get all interface nodes that
// have this VRFID. The Contrail routing table of these nodes is then
// updated according to the route update.
//
// LIMITATION: if the Contrail Vrouter Agent is restated, Skydive
// routing tables are corrupted. Skydive agent then have to be
// restarted when Contrail Vrouter agent is restarted.

package opencontrail

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// This represents the data we get from rt --monitor stdout
// easyjson:json
type rtMonitorRoute struct {
	Operation string
	Family    string
	VrfId     int `json:"vrf_id"`
	Prefix    int
	Address   string
	NhId      int `json:"nh_id"`
}

const afInetFamily string = "AF_INET"

const RouteProtocol int64 = 200

type interfaceUpdate struct {
	InterfaceUUID string
	VrfId         int
}

type routingTableUpdateType int

const (
	AddRoute routingTableUpdateType = iota
	DelRoute
	AddInterface
	DelInterface
)

type RoutingTableUpdate struct {
	action routingTableUpdateType
	route  rtMonitorRoute
	intf   interfaceUpdate
}

// routingTableUpdater serializes route update on both routing tables
// and interfaces.
func (p *Probe) routingTableUpdater() {
	var vrfId int
	p.Ctx.Logger.Debug("Starting routingTableUpdater...")
	for a := range p.routingTableUpdaterChan {
		switch a.action {
		case AddRoute:
			ocRoute := &Route{
				Protocol: RouteProtocol,
				Prefix:   fmt.Sprintf("%s/%d", a.route.Address, a.route.Prefix),
				Family:   a.route.Family,
				NhID:     int64(a.route.NhId)}
			p.addRoute(a.route.VrfId, ocRoute)
			vrfId = a.route.VrfId
		case DelRoute:
			ocRoute := &Route{
				Protocol: RouteProtocol,
				Prefix:   fmt.Sprintf("%s/%d", a.route.Address, a.route.Prefix),
				Family:   a.route.Family,
				NhID:     int64(a.route.NhId)}
			p.delRoute(a.route.VrfId, ocRoute)
			vrfId = a.route.VrfId
		case AddInterface:
			p.addInterface(a.intf.VrfId, a.intf.InterfaceUUID)
			vrfId = a.intf.VrfId
		case DelInterface:
			var err error
			if vrfId, err = p.deleteInterface(a.intf.InterfaceUUID); err != nil {
				continue
			}
		}
		p.onRouteChanged(vrfId)
	}
}

func (p *Probe) getOrCreateRoutingTable(vrfId int) *RoutingTable {
	vrf, exists := p.routingTables[vrfId]
	if !exists {
		p.Ctx.Logger.Debugf("Creating a new VRF with ID %d", vrfId)

		var err error
		if vrf, err = p.vrfInit(vrfId); err != nil {
			p.Ctx.Logger.Error(err)
			return nil
		}
	}
	return vrf
}

func (p *Probe) addInterface(vrfId int, interfaceUUID string) {
	if vrf := p.getOrCreateRoutingTable(vrfId); vrf != nil {
		p.Ctx.Logger.Debugf("Appending interface %s to VRF %d...", interfaceUUID, vrfId)
		vrf.InterfacesUUID = append(vrf.InterfacesUUID, interfaceUUID)
	}
}

func (p *Probe) OnInterfaceAdded(vrfId int, interfaceUUID string) {
	p.routingTableUpdaterChan <- RoutingTableUpdate{
		action: AddInterface,
		intf:   interfaceUpdate{InterfaceUUID: interfaceUUID, VrfId: vrfId},
	}
}

// deleteInterface removes interfaces from Vrf. If a Vrf no longer has
// any interfaces, this Vrf is removed.
func (p *Probe) deleteInterface(interfaceUUID string) (vrfId int, err error) {
	var found bool
	for k, vrf := range p.routingTables {
		for idx, intf := range vrf.InterfacesUUID {
			if intf == interfaceUUID {
				p.Ctx.Logger.Debugf("Delete interface %s from VRF %d", interfaceUUID, k)
				vrf.InterfacesUUID[idx] = vrf.InterfacesUUID[len(vrf.InterfacesUUID)-1]
				vrf.InterfacesUUID = vrf.InterfacesUUID[:len(vrf.InterfacesUUID)-1]
				found = true
				break
			}
		}
		if found {
			if len(vrf.InterfacesUUID) == 0 {
				p.Ctx.Logger.Debugf("Delete VRF %d", k)
				delete(p.routingTables, k)
			}
		}
	}
	return 0, errors.New("No VrfId was found")
}

func (p *Probe) OnInterfaceDeleted(interfaceUUID string) {
	p.routingTableUpdaterChan <- RoutingTableUpdate{
		action: DelInterface,
		intf:   interfaceUpdate{InterfaceUUID: interfaceUUID},
	}
}

// onRouteChanged writes the Contrail routing table into the
// Contrail.RoutingTable metadata attribute.
func (p *Probe) onRouteChanged(vrfId int) {
	vrf := p.getOrCreateRoutingTable(vrfId)

	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	filter := graph.NewElementFilter(filters.NewTermInt64Filter("Contrail.VRFID", int64(vrfId)))
	intfs := p.Ctx.Graph.GetNodes(filter)

	if len(intfs) == 0 {
		p.Ctx.Logger.Debugf("No interface with VRF index %d was found (on route add)", vrfId)
		return
	}

	for _, n := range intfs {
		contrailField, err := n.GetField("Contrail")
		if err != nil {
			continue
		}

		if metadata, ok := contrailField.(*Metadata); ok {
			metadata.RoutingTable = vrf.Routes
			p.Ctx.Graph.AddMetadata(n, "Contrail", metadata)
			p.Ctx.Logger.Debugf("Update routes on node %s", n.ID)
		}
	}
}

func (p *Probe) addRoute(vrfId int, route *Route) {
	if vrf := p.getOrCreateRoutingTable(vrfId); vrf != nil {
		p.Ctx.Logger.Debugf("Adding route %v to vrf %d", route, vrfId)
		for _, r := range vrf.Routes {
			if r == route {
				return
			}
		}
		vrf.Routes = append(vrf.Routes, route)
	}
}

func (p *Probe) delRoute(vrfId int, route *Route) {
	if vrf := p.getOrCreateRoutingTable(vrfId); vrf != nil {
		for i, r := range vrf.Routes {
			if r.Prefix == route.Prefix {
				p.Ctx.Logger.Debugf("Removing route %s from vrf %d ", r.Prefix, vrfId)
				vrf.Routes[i] = vrf.Routes[len(vrf.Routes)-1]
				vrf.Routes = vrf.Routes[:len(vrf.Routes)-1]
				return
			}
		}
		p.Ctx.Logger.Errorf("Can not remove route %v from vrf %d because route has not been found", route, vrfId)
	}
}

// vrfInit uses the Contrail binary rt --dump to get all routes of a VRF.
func (p *Probe) vrfInit(vrfId int) (*RoutingTable, error) {
	p.Ctx.Logger.Debugf("Initialization of VRF %d...", vrfId)

	cmd := exec.Command("rt", "--dump", fmt.Sprint(vrfId))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer cmd.Wait()

	scanner := bufio.NewScanner(stdout)
	separator := regexp.MustCompile("[[:space:]]+")

	// Remove the rt --dump stdout header
	scanner.Scan()
	scanner.Scan()
	scanner.Scan()

	vrf := &RoutingTable{}
	for scanner.Scan() {
		s := separator.Split(scanner.Text(), -1)
		// Ignore non complete entries
		if len(s) != 6 {
			continue
		}

		nhID, err := strconv.Atoi(s[4])
		if err != nil {
			return nil, err
		}
		// These are not interesting routes
		if nhID == 0 || nhID == 1 {
			continue
		}

		// TODO add family
		vrf.Routes = append(vrf.Routes, &Route{
			Protocol: RouteProtocol,
			Prefix:   s[0],
			NhID:     int64(nhID),
			Family:   afInetFamily,
		})
	}

	p.routingTables[vrfId] = vrf
	return vrf, nil
}

// We use the binary program "rt" that comes with Contrail to get
// notifications on Contrail route creations and deletions. These
// notifications are broadcasted with Netlink by the linux kernel
// Contrail module. We cannot just listen the Netlink bus because
// messages are encoded with Sandesh which is bound to the Contrail
// version. This is why we read the stdout of the "rt" tools.
func (p *Probe) rtMonitor() {
	cmd := exec.CommandContext(p.cancelCtx, "rt", "--monitor")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.Ctx.Logger.Error(err)
		return
	}
	stdoutBuf := bufio.NewReader(stdout)

	p.Ctx.Logger.Debugf("Starting OpenContrail route monitor")
	if err := cmd.Start(); err != nil {
		p.Ctx.Logger.Error(err)
		return
	}
	defer p.Ctx.Logger.Debugf("Stopping OpenContrail route monitor")

	go p.routingTableUpdater()

	var route rtMonitorRoute
	for {
		line, err := stdoutBuf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				p.Ctx.Logger.Errorf("Failed to read 'rt --monitor' output: %s", err)
			}
			return
		}
		if err := json.Unmarshal([]byte(line), &route); err != nil {
			p.Ctx.Logger.Error(err)
			continue
		}
		// We currently only support IPV4 routes
		if route.Family != afInetFamily {
			continue
		}
		switch route.Operation {
		case "add":
			p.Ctx.Logger.Debugf("Route add %v", route)
			p.routingTableUpdaterChan <- RoutingTableUpdate{action: AddRoute, route: route}
		case "delete":
			p.Ctx.Logger.Debugf("Route delete %v", route)
			p.routingTableUpdaterChan <- RoutingTableUpdate{action: DelRoute, route: route}
		}
	}
}
