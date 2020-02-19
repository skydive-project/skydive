/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package client

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"
	"github.com/spf13/cobra"
)

var (
	bpfFilter          string
	captureName        string
	captureDescription string
	captureType        string
	captureTTL         uint64
	nodeTID            string
	port               int
	samplingRate       uint32
	pollingInterval    uint32
	headerSize         int
	rawPacketLimit     int
	extraTCPMetric     bool
	ipDefrag           bool
	reassembleTCP      bool
	layerKeyMode       string
	extraLayers        []string
	target             string
	targetType         string
)

// CaptureCmd skydive capture root command
var CaptureCmd = &cobra.Command{
	Use:          "capture",
	Short:        "Manage captures",
	Long:         "Manage captures",
	SilenceUsage: false,
}

// CaptureCreate skydive capture creates command
var CaptureCreate = &cobra.Command{
	Use:   "create",
	Short: "Create capture",
	Long:  "Create capture",
	PreRun: func(cmd *cobra.Command, args []string) {
		if nodeTID != "" {
			if gremlinQuery != "" {
				exitOnError(errors.New("Options --node and --gremlin are exclusive"))
			}
			gremlinQuery = fmt.Sprintf("g.V().Has('TID', '%s')", nodeTID)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		var layers flow.ExtraLayers
		if err := layers.Parse(extraLayers...); err != nil {
			exitOnError(err)
		}

		capture := api.NewCapture(gremlinQuery, bpfFilter)
		capture.Name = captureName
		capture.Description = captureDescription
		capture.Type = captureType
		capture.Port = port
		capture.SamplingRate = samplingRate
		capture.PollingInterval = pollingInterval
		capture.HeaderSize = headerSize
		capture.ExtraTCPMetric = extraTCPMetric
		capture.IPDefrag = ipDefrag
		capture.ReassembleTCP = reassembleTCP
		capture.LayerKeyMode = layerKeyMode
		capture.RawPacketLimit = rawPacketLimit
		capture.ExtraLayers = int(layers)
		capture.Target = target
		capture.TargetType = targetType

		if err := validator.Validate("capture", capture); err != nil {
			exitOnError(err)
		}

		var createOpts *http.CreateOptions
		if captureTTL != 0 {
			createOpts = &http.CreateOptions{
				TTL: time.Duration(captureTTL) * time.Millisecond,
			}
		}

		if err := client.Create("capture", &capture, createOpts); err != nil {
			exitOnError(err)
		}
		printJSON(&capture)
	},
}

// CaptureList skydive capture list command
var CaptureList = &cobra.Command{
	Use:   "list",
	Short: "List captures",
	Long:  "List captures",
	Run: func(cmd *cobra.Command, args []string) {
		var captures map[string]api.Capture
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.List("capture", &captures); err != nil {
			exitOnError(err)
		}
		printJSON(captures)
	},
}

// CaptureGet skydive capture get command
var CaptureGet = &cobra.Command{
	Use:   "get [capture]",
	Short: "Display capture",
	Long:  "Display capture",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var capture api.Capture
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Get("capture", args[0], &capture); err != nil {
			exitOnError(err)
		}
		printJSON(&capture)
	},
}

// CaptureDelete skydive capture delete command
var CaptureDelete = &cobra.Command{
	Use:   "delete [capture]",
	Short: "Delete capture",
	Long:  "Delete capture",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		for _, id := range args {
			if err := client.Delete("capture", id); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

func addCaptureFlags(cmd *cobra.Command) {
	helpText := fmt.Sprintf("Allowed capture types: %v", probes.ProbeTypes)
	cmd.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Gremlin Query")
	cmd.Flags().StringVarP(&nodeTID, "node", "", "", "node TID")
	cmd.Flags().StringVarP(&bpfFilter, "bpf", "", "", "BPF filter")
	cmd.Flags().StringVarP(&captureName, "name", "", "", "capture name")
	cmd.Flags().StringVarP(&captureDescription, "description", "", "", "capture description")
	cmd.Flags().StringVarP(&captureType, "type", "", "", helpText)
	cmd.Flags().IntVarP(&port, "port", "", 0, "capture port")
	cmd.Flags().Uint32VarP(&samplingRate, "samplingrate", "", 1, "sampling Rate for SFlow Flow Sampling, 0 - no flow samples, default: 1")
	cmd.Flags().Uint32VarP(&pollingInterval, "pollinginterval", "", 10, "polling Interval for SFlow Counter Sampling, 0 - no counter samples, default: 10")
	cmd.Flags().IntVarP(&headerSize, "header-size", "", 0, fmt.Sprintf("header size of packet used, default: %d", flow.MaxCaptureLength))
	cmd.Flags().IntVarP(&rawPacketLimit, "rawpacket-limit", "", 0, "set the limit of raw packet captured, 0 no packet, -1 infinite, default: 0")
	cmd.Flags().BoolVarP(&extraTCPMetric, "extra-tcp-metric", "", false, "add additional TCP metric to flows, default: false")
	cmd.Flags().BoolVarP(&ipDefrag, "ip-defrag", "", false, "defragment IPv4 packets, default: false")
	cmd.Flags().BoolVarP(&reassembleTCP, "reassamble-tcp", "", false, "reassemble TCP packets, default: false")
	cmd.Flags().StringVarP(&layerKeyMode, "layer-key-mode", "", "L2", "defines the first layer used by flow key calculation, L2 or L3")
	cmd.Flags().StringArrayVarP(&extraLayers, "extra-layer", "", []string{}, fmt.Sprintf("list of extra layers to be added to the flow, available: %s", flow.ExtraLayers(flow.ALLLayer)))
	cmd.Flags().StringVarP(&target, "target", "", "", "sFlow/NetFlow target, if empty the agent will be used")
	cmd.Flags().StringVarP(&targetType, "target-type", "", "", "target type (netflowv5, erspanv1), ignored in case of sFlow/NetFlow capture")
	cmd.Flags().Uint64VarP(&captureTTL, "ttl", "", 0, "capture duration in milliseconds")
}

func init() {
	CaptureCmd.AddCommand(CaptureList)
	CaptureCmd.AddCommand(CaptureCreate)
	CaptureCmd.AddCommand(CaptureGet)
	CaptureCmd.AddCommand(CaptureDelete)

	addCaptureFlags(CaptureCreate)
}
