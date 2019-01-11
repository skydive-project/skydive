/*
 * Copyright (C) 2017 Orange.
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

package ovsdb

import (
	"context"
	"fmt"
	"sync"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// OvsOfProbe is the type of the probe retrieving Openflow rules on an Open Vswitch
type OvsOfProbe struct {
	sync.Mutex
	Host           string                    // The host
	Graph          *graph.Graph              // The graph that will receive the rules found
	Root           *graph.Node               // The root node of the host in the graph
	bridgeOfProbes map[string]*bridgeOfProbe // The table of probes associated to each bridge
	Translation    map[string]string         // A translation table to find the url for a given bridge knowing its name
	Certificate    string                    // Path to the certificate used for authenticated communication with bridges
	PrivateKey     string                    // Path of the private key authenticating the probe.
	CA             string                    // Path of the certicate of the Certificate authority used for authenticated communication with bridges
	sslOk          bool                      // cert private key and ca are provisionned.
	useNative      bool
	ctx            context.Context
}

// BridgeOfProber is the type of the probe retrieving Openflow rules on a Bridge.
type BridgeOfProber interface {
	Monitor(ctx context.Context) error
	MonitorGroup() error
}

type bridgeOfProbe struct {
	cancel context.CancelFunc
	prober BridgeOfProber
}

// newbridgeOfProbe creates a probe and launch the active process
func (o *OvsOfProbe) newbridgeOfProbe(host string, bridge string, uuid string, bridgeNode *graph.Node) (*bridgeOfProbe, error) {
	ctx, cancel := context.WithCancel(o.ctx)
	address, ok := o.Translation[bridge]
	if !ok {
		logging.GetLogger().Warningf("Could not find translation address for %s in %v", bridge, o.Translation)
		address = fmt.Sprintf("unix:/var/run/openvswitch/%s.mgmt", bridge)
	}

	var prober BridgeOfProber
	if o.useNative {
		prober = NewOfProbe(bridge, address, o.Graph, bridgeNode)
	} else {
		prober = NewOfctlProbe(host, bridge, uuid, address, bridgeNode, o)
	}

	if err := prober.Monitor(ctx); err != nil {
		logging.GetLogger().Error(err)
		cancel()
		return nil, err
	}

	if err := prober.MonitorGroup(); err != nil {
		logging.GetLogger().Errorf("Cannot add group probe on %s - %s", bridge, err)
	}

	return &bridgeOfProbe{cancel: cancel, prober: prober}, nil
}

// OnOvsBridgeAdd is called when a bridge is added
func (o *OvsOfProbe) OnOvsBridgeAdd(bridgeNode *graph.Node) {
	o.Lock()
	defer o.Unlock()

	uuid, _ := bridgeNode.GetFieldString("UUID")
	if probe, ok := o.bridgeOfProbes[uuid]; ok {
		if err := probe.prober.MonitorGroup(); err != nil {
			logging.GetLogger().Debug(err)
		}
		return
	}

	bridgeName, _ := bridgeNode.GetFieldString("Name")
	bridgeOfProbe, err := o.newbridgeOfProbe(o.Host, bridgeName, uuid, bridgeNode)
	if err != nil {
		logging.GetLogger().Errorf("Cannot add probe for bridge %s on %s (%s): %s", bridgeName, o.Host, uuid, err)
		return
	}

	logging.GetLogger().Debugf("Probe added for %s on %s (%s)", bridgeName, o.Host, uuid)
	o.bridgeOfProbes[uuid] = bridgeOfProbe
}

// OnOvsBridgeDel is called when a bridge is deleted
func (o *OvsOfProbe) OnOvsBridgeDel(uuid string) {
	o.Lock()
	defer o.Unlock()

	if bridgeOfProbe, ok := o.bridgeOfProbes[uuid]; ok {
		bridgeOfProbe.cancel()
		delete(o.bridgeOfProbes, uuid)
	}

	// Clean all the rules attached to the bridge.
	o.Graph.Lock()
	defer o.Graph.Unlock()

	bridgeNode := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridgeNode != nil {
		rules := o.Graph.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofrule"}, nil)
		for _, ruleNode := range rules {
			logging.GetLogger().Infof("Rule %v deleted (Bridge deleted)", ruleNode.Metadata["UUID"])
			if err := o.Graph.DelNode(ruleNode); err != nil {
				logging.GetLogger().Error(err)
			}
		}

		groups := o.Graph.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofgroup"}, nil)
		for _, groupNode := range groups {
			logging.GetLogger().Infof("Group %v deleted (Bridge deleted)", groupNode.Metadata["UUID"])
			if err := o.Graph.DelNode(groupNode); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	}
}

// NewOvsOfProbe creates a new probe associated to a given graph, root node and host.
func NewOvsOfProbe(ctx context.Context, g *graph.Graph, root *graph.Node, host string) *OvsOfProbe {
	if !config.GetBool("ovs.oflow.enable") {
		return nil
	}

	logging.GetLogger().Infof("Adding OVS probe on %s", host)

	translate := config.GetStringMapString("ovs.oflow.address")
	cert := config.GetString("ovs.oflow.cert")
	pk := config.GetString("ovs.oflow.key")
	ca := config.GetString("ovs.oflow.ca")
	sslOk := (pk != "") && (ca != "") && (cert != "")
	useNative := config.GetBool("ovs.oflow.native")

	return &OvsOfProbe{
		Host:           host,
		Graph:          g,
		Root:           root,
		bridgeOfProbes: make(map[string]*bridgeOfProbe),
		Translation:    translate,
		Certificate:    cert,
		PrivateKey:     pk,
		CA:             ca,
		sslOk:          sslOk,
		useNative:      useNative,
		ctx:            ctx,
	}
}
