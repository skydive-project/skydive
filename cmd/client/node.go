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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	usertopology "github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	host           string
	gremlinFlag    bool
	addMetadata    []string
	removeMetadata []string
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
		restClient, err := client.NewRestClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		var ids []string
		if gremlinFlag {
			queryHelper := client.NewGremlinQueryHelper(restClient)

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

		crudClient := http.NewCrudClient(restClient)
		for _, arg := range ids {
			if err := crudClient.Delete("node", arg); err != nil {
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
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		patch, err := createMetadataJSONPatch()
		if err != nil {
			exitOnError(err)
		}

		var result interface{}
		for _, id := range args {
			if _, err := client.Update("node", id, patch, &result); err != nil {
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

func createMetadataJSONPatch() (patch JSONPatch, _ error) {
	for _, add := range addMetadata {
		split := strings.SplitN(add, "=", 2)
		if len(split) < 2 {
			return nil, fmt.Errorf("metadata to add should be of the form k1=v1, got %s", add)
		}

		var value interface{}
		if err := json.Unmarshal([]byte(split[1]), &value); err != nil {
			value = split[1]
		}
		patch = append(patch, newPatchOperation("add", "/Metadata/"+strings.Replace(split[0], "/", ".", -1), value))
	}

	for _, remove := range removeMetadata {
		patch = append(patch, newPatchOperation("remove", "/Metadata/"+strings.Replace(remove, "/", ".", -1)))
	}

	return patch, nil
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

	addCreateNodeFlags(NodeCreate)
	NodeCmd.AddCommand(NodeCreate)

	NodeDelete.Flags().BoolVarP(&gremlinFlag, "gremlin", "", false, "use Gremlin expressions instead of a node identifiers")
	NodeCmd.AddCommand(NodeDelete)

	NodeUpdate.Flags().StringArrayVarP(&addMetadata, "add", "a", nil, "node metadata to add, key value pair. 'k1=v1'")
	NodeUpdate.Flags().StringArrayVarP(&removeMetadata, "remove", "r", nil, "node metadata keys to remove")
	NodeCmd.AddCommand(NodeUpdate)
}
