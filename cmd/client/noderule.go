/*
 * Copyright (C) 2018 Red Hat, Inc.
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

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	usertopology "github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	name        string
	description string
	nodeName    string
	nodeType    string
	metadata    string
	query       string
	action      string
)

// NodeRuleCmd skydive node rule root command
var NodeRuleCmd = &cobra.Command{
	Use:          "node-rule",
	Short:        "node-rule",
	Long:         "node-rule",
	SilenceUsage: false,
}

// NodeRuleCreate skydive node create command
var NodeRuleCreate = &cobra.Command{
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

		if action == "create" {
			if nodeName == "" || nodeType == "" {
				exitOnError(errors.New("Both --node-name and --node-type are required for 'create' node rules"))
			}

			m["Name"] = nodeName
			m["Type"] = nodeType
		}

		nodeRule := &api.NodeRule{
			Name:        name,
			Description: description,
			Metadata:    m,
			Query:       query,
			Action:      action,
		}

		if err = validator.Validate("noderule", nodeRule); err != nil {
			exitOnError(fmt.Errorf("Error while validating node rule: %s", err))
		}

		if err = client.Create("noderule", &nodeRule, nil); err != nil {
			exitOnError(err)
		}

		printJSON(nodeRule)
	},
}

// NodeRuleGet skydive node get command
var NodeRuleGet = &cobra.Command{
	Use:          "get",
	Short:        "get",
	Long:         "get",
	SilenceUsage: false,

	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		var node api.NodeRule
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}
		if err := client.Get("noderule", args[0], &node); err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}
		printJSON(&node)
	},
}

// NodeRuleList skydive node list command
var NodeRuleList = &cobra.Command{
	Use:          "list",
	Short:        "list",
	Long:         "list",
	SilenceUsage: false,

	Run: func(cmd *cobra.Command, args []string) {
		var nodes map[string]api.NodeRule
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}

		if err := client.List("noderule", &nodes); err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}
		printJSON(nodes)
	},
}

// NodeRuleDelete skydive node delete command
var NodeRuleDelete = &cobra.Command{
	Use:          "delete",
	Short:        "delete",
	Long:         "delete",
	SilenceUsage: false,

	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}

		for _, id := range args {
			if err := client.Delete("noderule", id); err != nil {
				logging.GetLogger().Error(err.Error())
			}
		}
	},
}

func addCreateNodeRuleFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&name, "name", "", "", "rule name")
	cmd.Flags().StringVarP(&description, "description", "", "", "rule description")
	cmd.Flags().StringVarP(&nodeName, "node-name", "", "", "node name")
	cmd.Flags().StringVarP(&nodeType, "node-type", "", "", "node type")
	cmd.Flags().StringVarP(&metadata, "metadata", "", "", "node metadata, key value pairs. 'k1=v1, k2=v2'")
	cmd.Flags().StringVarP(&query, "query", "", "", "gremlin query of the nodes to update")
	cmd.Flags().StringVarP(&action, "action", "", "", "action: create or update")
}

func init() {
	NodeRuleCmd.AddCommand(NodeRuleCreate)
	NodeRuleCmd.AddCommand(NodeRuleList)
	NodeRuleCmd.AddCommand(NodeRuleGet)
	NodeRuleCmd.AddCommand(NodeRuleDelete)

	addCreateNodeRuleFlags(NodeRuleCreate)
}
