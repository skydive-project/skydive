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
	"crypto/tls"
	"errors"
	"fmt"
	"sync"

	"github.com/skydive-project/skydive/graffiti/graph"
	gtls "github.com/skydive-project/skydive/graffiti/tls"
	tp "github.com/skydive-project/skydive/topology/probes"
)

// OvsOfProbeHandler is the type of the probe retrieving Openflow rules on an Open Vswitch
type OvsOfProbeHandler struct {
	sync.Mutex
	host           string                    // The host
	protocol       string                    // Protocol to use to talk to the switch
	Ctx            tp.Context                // Probe context
	bridgeOfProbes map[string]*bridgeOfProbe // The table of probes associated to each bridge
	translation    map[string]string         // A translation table to find the url for a given bridge knowing its name
	certificate    string                    // Path to the certificate used for authenticated communication with bridges
	privateKey     string                    // Path of the private key authenticating the probe.
	ca             string                    // Path of the certicate of the Certificate authority used for authenticated communication with bridges
	sslOk          bool                      // cert private key and ca are provisionned.
	tlsConfig      *tls.Config
	useNative      bool
	cancelCtx      context.Context
}

// BridgeOfProber is the type of the probe retrieving Openflow rules on a Bridge.
type BridgeOfProber interface {
	Monitor(ctx context.Context) error
	MonitorGroup() error
}

type bridgeOfProbe struct {
	cancelFunc context.CancelFunc
	prober     BridgeOfProber
}

var (
	// ErrGroupNotSupported is reported when group monitoring is not supported by ovs
	ErrGroupNotSupported = errors.New("Group monitoring is only possible on OpenFlow 1.5 and later because of an OVS bug")
)

// newbridgeOfProbe creates a probe and launch the active process
func (o *OvsOfProbeHandler) newbridgeOfProbe(bridge string, uuid string, bridgeNode *graph.Node) (*bridgeOfProbe, error) {
	address, ok := o.translation[bridge]
	if !ok {
		if o.protocol != "unix" {
			return nil, fmt.Errorf("Could not find translation unix address for %s in %v", bridge, o.translation)
		}
		address = fmt.Sprintf("unix:/var/run/openvswitch/%s.mgmt", bridge)
	}

	cancelCtx, cancelFunc := context.WithCancel(o.cancelCtx)

	ctx := tp.Context{
		Logger:   o.Ctx.Logger,
		Config:   o.Ctx.Config,
		Graph:    o.Ctx.Graph,
		RootNode: bridgeNode,
	}

	var prober BridgeOfProber
	if o.useNative {
		prober = NewOfProbe(ctx, bridge, address, o.tlsConfig)
	} else {
		prober = NewOfctlProbe(ctx, o.host, bridge, uuid, address, o)
	}

	if err := prober.Monitor(cancelCtx); err != nil {
		o.Ctx.Logger.Error(err)
		cancelFunc()
		return nil, err
	}

	if err := prober.MonitorGroup(); err != nil {
		if err == ErrGroupNotSupported {
			o.Ctx.Logger.Warningf("Cannot add group probe on %s - %s", bridge, err)
		} else {
			o.Ctx.Logger.Errorf("Cannot add group probe on %s - %s", bridge, err)
		}
	}

	return &bridgeOfProbe{cancelFunc: cancelFunc, prober: prober}, nil
}

// OnOvsBridgeAdd is called when a bridge is added
func (o *OvsOfProbeHandler) OnOvsBridgeAdd(bridgeNode *graph.Node) {
	o.Lock()
	defer o.Unlock()

	uuid, _ := bridgeNode.GetFieldString("UUID")
	bridgeName, _ := bridgeNode.GetFieldString("Name")

	if probe, ok := o.bridgeOfProbes[uuid]; ok {
		if err := probe.prober.MonitorGroup(); err != nil {
			if err == ErrGroupNotSupported {
				o.Ctx.Logger.Warningf("Cannot add group probe on %s - %s", bridgeName, err)
			} else {
				o.Ctx.Logger.Errorf("Cannot add group probe on %s - %s", bridgeName, err)
			}
		}
		return
	}

	bridgeOfProbe, err := o.newbridgeOfProbe(bridgeName, uuid, bridgeNode)
	if err != nil {
		return
	}

	o.Ctx.Logger.Debugf("Probe added for %s on %s (%s)", bridgeName, o.host, uuid)
	o.bridgeOfProbes[uuid] = bridgeOfProbe
}

// OnOvsBridgeDel is called when a bridge is deleted
func (o *OvsOfProbeHandler) OnOvsBridgeDel(uuid string) {
	o.Lock()
	defer o.Unlock()

	if bridgeOfProbe, ok := o.bridgeOfProbes[uuid]; ok {
		bridgeOfProbe.cancelFunc()
		delete(o.bridgeOfProbes, uuid)
	}

	// Clean all the rules attached to the bridge.
	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()

	bridgeNode := o.Ctx.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridgeNode != nil {
		rules := o.Ctx.Graph.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofrule"}, nil)
		for _, ruleNode := range rules {
			o.Ctx.Logger.Infof("Rule %v deleted (Bridge deleted)", ruleNode.Metadata["UUID"])
			if err := o.Ctx.Graph.DelNode(ruleNode); err != nil {
				o.Ctx.Logger.Error(err)
			}
		}

		groups := o.Ctx.Graph.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofgroup"}, nil)
		for _, groupNode := range groups {
			o.Ctx.Logger.Infof("Group %v deleted (Bridge deleted)", groupNode.Metadata["UUID"])
			if err := o.Ctx.Graph.DelNode(groupNode); err != nil {
				o.Ctx.Logger.Error(err)
			}
		}
	}
}

// NewOvsOfProbeHandler creates a new probe associated to a given graph, root node and host.
func NewOvsOfProbeHandler(cancelCtx context.Context, ctx tp.Context, host string, protocol string) (*OvsOfProbeHandler, error) {
	if !ctx.Config.GetBool("ovs.oflow.enable") {
		return nil, nil
	}

	ctx.Logger.Infof("Adding OVS probe on %s", host)

	translate := ctx.Config.GetStringMapString("ovs.oflow.address")
	cert := ctx.Config.GetString("ovs.oflow.cert")
	pk := ctx.Config.GetString("ovs.oflow.key")
	ca := ctx.Config.GetString("ovs.oflow.ca")
	sslOk := (pk != "") && (ca != "") && (cert != "")
	useNative := ctx.Config.GetBool("ovs.oflow.native")

	var err error
	var tlsConfig *tls.Config
	if useNative && sslOk {
		tlsConfig, err = gtls.SetupTLSClientConfig(cert, pk)
		if err != nil {
			return nil, err
		}
		if tlsConfig.RootCAs, err = gtls.SetupTLSLoadCA(ca); err != nil {
			return nil, err
		}
	}

	return &OvsOfProbeHandler{
		host:           host,
		protocol:       protocol,
		Ctx:            ctx,
		bridgeOfProbes: make(map[string]*bridgeOfProbe),
		translation:    translate,
		certificate:    cert,
		privateKey:     pk,
		ca:             ca,
		tlsConfig:      tlsConfig,
		sslOk:          sslOk,
		useNative:      useNative,
		cancelCtx:      cancelCtx,
	}, nil
}
