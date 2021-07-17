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

	"github.com/skydive-project/skydive/graffiti/api/client"
	"github.com/skydive-project/skydive/graffiti/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"

	"github.com/spf13/cobra"
)

var (
	gremlinFlag        bool
	nodeMetadata       string
	nodeName           string
	nodeType           string
	nodeAddMetadata    []string
	nodeRemoveMetadata []string
)

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
		m, err := DefToMetadata(nodeMetadata, graph.Metadata{})
		if err != nil {
			exitOnError(err)
		}

		if nodeName != "" {
			m["Name"] = nodeName
		}

		if nodeType != "" {
			m["Type"] = nodeType
		}

		origin := graph.Origin(Host, CLIService)
		node := &types.Node{graph.CreateNode(graph.GenID(), m, graph.Time(time.Now()), Host, origin)}

		if err = CrudClient.Create("node", node, nil); err != nil {
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
		if err := CrudClient.List("node", &nodes); err != nil {
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
		if err := CrudClient.Get("node", args[0], &node); err != nil {
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
		var ids []string
		if gremlinFlag {
			queryHelper := client.NewGremlinQueryHelper(CrudClient.RestClient)

			for _, gremlinQuery := range args {
				nodes, err := queryHelper.GetNodes(gremlinQuery)
				if err != nil {
					exitOnError(err)
				}

				for _, node := range nodes {
					ids = append(ids, string(node.ID))
				}
			}
		} else {
			ids = args
		}

		for _, arg := range ids {
			if err := CrudClient.Delete("node", arg); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

// NodeUpdate node delete command
var NodeUpdate = &cobra.Command{
	Use:   "update [node]",
	Short: "Update node",
	Long:  "Update node",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		patch, err := createMetadataJSONPatch(nodeAddMetadata, nodeRemoveMetadata)
		if err != nil {
			exitOnError(err)
		}

		var result interface{}
		for _, id := range args {
			if _, err := CrudClient.Update("node", id, patch, &result); err != nil {
				exitOnError(err)
			}
		}

		if result == nil {
			fmt.Println("Not modified")
		} else {
			printJSON(result)
		}
	},
}

func addCreateNodeFlags(cmd *cobra.Command) {
	Host, _ = os.Hostname()
	cmd.Flags().StringVarP(&nodeName, "node-name", "", "", "node name")
	cmd.Flags().StringVarP(&nodeType, "node-type", "", "", "node type")
	cmd.Flags().StringVarP(&nodeMetadata, "metadata", "", "", "node metadata, key value pairs. 'k1=v1, k2=v2'")
	cmd.Flags().StringVarP(&Host, "host", "", Host, "host")
}

func init() {
	NodeCmd.AddCommand(NodeList)
	NodeCmd.AddCommand(NodeGet)

	addCreateNodeFlags(NodeCreate)
	NodeCmd.AddCommand(NodeCreate)

	NodeDelete.Flags().BoolVarP(&gremlinFlag, "gremlin", "", false, "use Gremlin expressions instead of a node identifiers")
	NodeCmd.AddCommand(NodeDelete)

	NodeUpdate.Flags().StringArrayVarP(&nodeAddMetadata, "add", "a", nil, "node metadata to add, key value pair. 'k1=v1'")
	NodeUpdate.Flags().StringArrayVarP(&nodeRemoveMetadata, "remove", "r", nil, "node metadata keys to remove")
	NodeCmd.AddCommand(NodeUpdate)
}
