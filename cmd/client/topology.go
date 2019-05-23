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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/hub"
	gws "github.com/skydive-project/skydive/graffiti/websocket"
	"github.com/skydive-project/skydive/websocket"
	"github.com/spf13/cobra"
)

var (
	gremlinQuery string
	outputFormat string
	filename     string
)

// TopologyCmd skydive topology root command
var TopologyCmd = &cobra.Command{
	Use:          "topology",
	Short:        "Request on topology",
	Long:         "Request on topology",
	SilenceUsage: false,
}

// TopologyRequest skydive topology query command
var TopologyRequest = &cobra.Command{
	Use:   "query",
	Short: "query topology [deprecated: use 'client query' instead]",
	Long:  "query topology [deprecated: use 'client query' instead]",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "The 'client topology query' command is deprecated. Please use 'client query' instead")
		QueryCmd.Run(cmd, []string{gremlinQuery})
	},
}

// TopologyImport skydive topology import command
var TopologyImport = &cobra.Command{
	Use:   "import",
	Short: "import topology",
	Long:  "import topology",
	Run: func(cmd *cobra.Command, args []string) {
		sa, err := config.GetOneAnalyzerServiceAddress()
		if err != nil {
			exitOnError(err)
		}

		url := config.GetURL("ws", sa.Addr, sa.Port, "/ws/publisher")
		opts := websocket.ClientOpts{AuthOpts: &AuthenticationOpts, Headers: http.Header{}}
		opts.Headers.Add("X-Persistence-Policy", string(hub.Persistent))
		client, err := config.NewWSClient(common.UnknownService, url, opts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Connect(); err != nil {
			exitOnError(err)
		}

		go client.Run()
		defer func() {
			client.Flush()
			client.StopAndWait()
		}()

		content, err := ioutil.ReadFile(filename)
		if err != nil {
			exitOnError(err)
		}

		els := new(graph.Elements)
		if err := json.Unmarshal(content, els); err != nil {
			exitOnError(err)
		}

		for _, node := range els.Nodes {
			msg := gws.NewStructMessage(gws.NodeAddedMsgType, node)
			if err := client.SendMessage(msg); err != nil {
				exitOnError(fmt.Errorf("Failed to send message: %s", err))
			}
		}

		for _, edge := range els.Edges {
			msg := gws.NewStructMessage(gws.EdgeAddedMsgType, edge)
			if err := client.SendMessage(msg); err != nil {
				exitOnError(fmt.Errorf("Failed to send message: %s", err))
			}
		}
	},
}

// TopologyExport skydive topology export command
var TopologyExport = &cobra.Command{
	Use:   "export",
	Short: "export topology",
	Long:  "export topology",
	Run: func(cmd *cobra.Command, args []string) {
		QueryCmd.Run(cmd, []string{"G"})
	},
}

func init() {
	TopologyCmd.AddCommand(TopologyExport)

	TopologyImport.Flags().StringVarP(&filename, "file", "", "graph.json", "Input file")
	TopologyCmd.AddCommand(TopologyImport)

	TopologyRequest.Flags().StringVarP(&gremlinQuery, "gremlin", "", "G", "Gremlin Query")
	TopologyRequest.Flags().StringVarP(&outputFormat, "format", "", "json", "Output format (json, dot or pcap)")
	TopologyCmd.AddCommand(TopologyRequest)
}
