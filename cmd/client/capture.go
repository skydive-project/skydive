/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package client

import (
	"fmt"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/http"
	"github.com/spf13/cobra"
)

var captureOpts = struct {
	BpfFilter          string `flag:"bpf,BPF filter"`
	CaptureName        string `flag:"name,capture name"`
	CaptureDescription string `flag:"description,capture description"`
	CaptureType        string
	GremlinQuery       string `flag:"gremlin,Gremlin Query"`
	NodeTID            string `flag:"node,node TID"`
	Port               int    `flag:"port,capture port"`
	HeaderSize         int    `flag:"header-size,Header size of packet used"`
	RawPacketLimit     int    `flag:"rawpacket-limit,Set the limit of raw packet captured, 0 no packet, -1 infinite, default: 0"`
	ExtraTCPMetric     bool   `flag:"extra-tcp-metric,Add additional TCP metric to flows, default: false"`
	IPDefrag           bool   `flag:"ip-defrag,Defragment IPv4 packets, default: false"`
	ReassembleTCP      bool   `flag:"reassamble-tcp,Reassemble TCP packets, default: false"`
	LayerKeyMode       string `flag:"layer-key-mode,Defines the first layer used by flow key calculation, L2 or L3"`
}{
	LayerKeyMode: "L2",
}

var captureCmd = crudRootCommand{
	Resource: "capture",

	Get: &getHandler{
		Run: func(client *http.CrudClient, id string) (types.Resource, error) {
			var capture types.Capture
			err := client.Get("capture", id, &capture)
			return &capture, err
		},
	},

	List: &listHandler{
		Run: func(client *http.CrudClient) (map[string]types.Resource, error) {
			var captures map[string]types.Capture
			if err := client.List("capture", &captures); err != nil {
				return nil, err
			}
			resources := make(map[string]types.Resource, len(captures))
			for _, capture := range captures {
				resources[capture.ID()] = &capture
			}
			return resources, nil
		},
	},

	Create: &createHandler{
		Flags: func(crud *crudRootCommand, c *cobra.Command) {
			crud.setFlags(c, &captureOpts)

			types := []string{}
			found := map[string]bool{}
			for _, v := range common.CaptureTypes {
				for _, t := range v.Allowed {
					if found[t] != true {
						found[t] = true
						types = append(types, t)
					}
				}
			}
			helpText := fmt.Sprintf("Allowed capture types: %v", types)
			c.Flags().StringVarP(&captureOpts.CaptureType, "type", "", "", helpText)
		},

		PreRun: func() error {
			if captureOpts.NodeTID != "" {
				if captureOpts.GremlinQuery != "" {
					return fmt.Errorf("Options --node and --gremlin are exclusive")
				}
				captureOpts.GremlinQuery = fmt.Sprintf("g.V().Has('TID', '%s')", captureOpts.NodeTID)
			}
			return nil
		},

		Run: func(client *http.CrudClient) (types.Resource, error) {
			capture := types.NewCapture(captureOpts.GremlinQuery, captureOpts.BpfFilter)
			capture.Name = captureOpts.CaptureName
			capture.Description = captureOpts.CaptureDescription
			capture.Type = captureOpts.CaptureType
			capture.Port = captureOpts.Port
			capture.HeaderSize = captureOpts.HeaderSize
			capture.ExtraTCPMetric = captureOpts.ExtraTCPMetric
			capture.IPDefrag = captureOpts.IPDefrag
			capture.ReassembleTCP = captureOpts.ReassembleTCP
			capture.LayerKeyMode = captureOpts.LayerKeyMode

			if !config.GetConfig().GetBool("analyzer.packet_capture_enabled") {
				capture.RawPacketLimit = 0
			} else {
				capture.RawPacketLimit = captureOpts.RawPacketLimit
			}

			return capture, nil
		},
	},
}
