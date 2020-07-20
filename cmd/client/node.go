/*
 * Copyright (C) 2020 Sylvain Baubeau
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
	"fmt"
	"os"
	"time"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	usertopology "github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var host string

// NodeCmd skydive node rule root command
var NodeCmd = &cobra.Command{
	Use:          "node",
	Short:        "node",
	Long:         "node",
	SilenceUsage: false,
}

// NodeCreate skydive node create command
var NodeCreate = &cobra.Command{
	Use:          "create",
	Short:        "create",
	Long:         "create",
	SilenceUsage: false,

	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		m, err := usertopology.DefToMetadata(metadata, graph.Metadata{})
		if err != nil {
			exitOnError(err)
		}

		if nodeName != "" {
			m["Name"] = nodeName
		}

		if nodeType != "" {
			m["Type"] = nodeType
		}

		node := api.Node(*graph.CreateNode(graph.GenID(), m, graph.Time(time.Now()), host, config.AgentService))

		if err = validator.Validate("node", &node); err != nil {
			exitOnError(fmt.Errorf("Error while validating node: %s", err))
		}

		if err = client.Create("node", &node, nil); err != nil {
			exitOnError(err)
		}

		printJSON(node)
	},
}

// NodeList node list command
var NodeList = &cobra.Command{
	Use:   "list",
	Short: "List nodes",
	Long:  "List nodes",
	Run: func(cmd *cobra.Command, args []string) {
		var nodes map[string]types.Node
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}
		if err := client.List("node", &nodes); err != nil {
			exitOnError(err)
		}
		printJSON(nodes)
	},
}

// NodeGet node get command
var NodeGet = &cobra.Command{
	Use:   "get [node]",
	Short: "Display node",
	Long:  "Display node",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var node types.Node
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Get("node", args[0], &node); err != nil {
			exitOnError(err)
		}
		printJSON(&node)
	},
}

// NodeDelete node delete command
var NodeDelete = &cobra.Command{
	Use:   "delete [node]",
	Short: "Delete node",
	Long:  "Delete node",
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
			if err := client.Delete("node", id); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

func addCreateNodeFlags(cmd *cobra.Command) {
	host, _ = os.Hostname()
	cmd.Flags().StringVarP(&nodeName, "node-name", "", "", "node name")
	cmd.Flags().StringVarP(&nodeType, "node-type", "", "", "node type")
	cmd.Flags().StringVarP(&metadata, "metadata", "", "", "node metadata, key value pairs. 'k1=v1, k2=v2'")
	cmd.Flags().StringVarP(&metadata, "host", "", host, "host")
}

func init() {
	NodeCmd.AddCommand(NodeList)
	NodeCmd.AddCommand(NodeGet)
	NodeCmd.AddCommand(NodeCreate)
	NodeCmd.AddCommand(NodeDelete)

	addCreateNodeFlags(NodeCreate)
}
