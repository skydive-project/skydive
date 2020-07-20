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
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	usertopology "github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	edgeType     string
	parentNodeID string
	childNodeID  string
)

// EdgeCmd skydive edge rule root command
var EdgeCmd = &cobra.Command{
	Use:          "edge",
	Short:        "edge",
	Long:         "edge",
	SilenceUsage: false,
}

// EdgeCreate skydive edge create command
var EdgeCreate = &cobra.Command{
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

		if edgeType != "" {
			m["RelationType"] = edgeType
		}

		var parentNode, childNode graph.Node
		if err := client.Get("node", parentNodeID, &parentNode); err != nil {
			exitOnError(fmt.Errorf("Could not find parent node: %s", err))
		}

		if err := client.Get("node", childNodeID, &childNode); err != nil {
			exitOnError(fmt.Errorf("Could not find child node: %s", err))
		}

		origin := graph.Origin(host, CLIService)
		edge := api.Edge(*graph.CreateEdge(graph.GenID(), &parentNode, &childNode, m, graph.Time(time.Now()), host, origin))

		if err = validator.Validate("edge", &edge); err != nil {
			exitOnError(fmt.Errorf("Error while validating edge: %s", err))
		}

		if err = client.Create("edge", &edge, nil); err != nil {
			exitOnError(err)
		}

		printJSON(edge)
	},
}

// EdgeList edge list command
var EdgeList = &cobra.Command{
	Use:   "list",
	Short: "List edges",
	Long:  "List edges",
	Run: func(cmd *cobra.Command, args []string) {
		var edges map[string]types.Edge
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}
		if err := client.List("edge", &edges); err != nil {
			exitOnError(err)
		}
		printJSON(edges)
	},
}

// EdgeGet edge get command
var EdgeGet = &cobra.Command{
	Use:   "get [edge]",
	Short: "Display edge",
	Long:  "Display edge",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var edge types.Edge
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Get("edge", args[0], &edge); err != nil {
			exitOnError(err)
		}
		printJSON(&edge)
	},
}

// EdgeDelete edge delete command
var EdgeDelete = &cobra.Command{
	Use:   "delete [edge]",
	Short: "Delete edge",
	Long:  "Delete edge",
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
				edges, err := queryHelper.GetEdges(gremlinQuery)
				if err != nil {
					exitOnError(err)
				}

				for _, edge := range edges {
					ids = append(ids, string(edge.ID))
				}
			}
		} else {
			ids = args
		}

		crudClient := http.NewCrudClient(restClient)
		for _, arg := range ids {
			if err := crudClient.Delete("edge", arg); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

// EdgeUpdate node delete command
var EdgeUpdate = &cobra.Command{
	Use:   "update [edge]",
	Short: "Update edge",
	Long:  "Update edge",
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
			if _, err := client.Update("edge", id, patch, &result); err != nil {
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

func addCreateEdgeFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&edgeType, "edge-type", "", "", "edge type")
	cmd.Flags().StringVarP(&parentNodeID, "parent", "", "", "parent node identifier")
	cmd.Flags().StringVarP(&childNodeID, "child", "", "", "child node identifier")
	cmd.Flags().StringVarP(&metadata, "metadata", "", "", "edge metadata, key value pairs. 'k1=v1, k2=v2'")
}

func init() {
	EdgeCmd.AddCommand(EdgeList)
	EdgeCmd.AddCommand(EdgeGet)

	addCreateEdgeFlags(EdgeCreate)
	EdgeCmd.AddCommand(EdgeCreate)

	EdgeDelete.Flags().BoolVarP(&gremlinFlag, "gremlin", "", false, "use Gremlin expressions instead of a node identifiers")
	EdgeCmd.AddCommand(EdgeDelete)

	EdgeUpdate.Flags().StringArrayVarP(&addMetadata, "add", "a", nil, "edge metadata to add, key value pair. 'k1=v1'")
	EdgeUpdate.Flags().StringArrayVarP(&removeMetadata, "remove", "r", nil, "edge metadata keys to remove")
	EdgeCmd.AddCommand(EdgeUpdate)
}
