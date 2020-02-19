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
	src          string
	dst          string
	relationType string
)

// EdgeRuleCmd skydive edge rule root command
var EdgeRuleCmd = &cobra.Command{
	Use:          "edge-rule",
	Short:        "edge-rule",
	Long:         "edge-rule",
	SilenceUsage: false,
}

// EdgeRuleCreate skydive edge create command
var EdgeRuleCreate = &cobra.Command{
	Use:          "create",
	Short:        "create",
	Long:         "create",
	SilenceUsage: false,
	PreRun: func(cmd *cobra.Command, args []string) {
		if relationType == "" {
			logging.GetLogger().Error("--relationtype is a required parameter")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		m, err := usertopology.DefToMetadata(metadata, graph.Metadata{})
		if err != nil {
			exitOnError(err)
		}
		m["RelationType"] = relationType

		edgeRule := &api.EdgeRule{
			Name:        name,
			Description: description,
			Src:         src,
			Dst:         dst,
			Metadata:    m,
		}

		if err = validator.Validate("edgerule", edgeRule); err != nil {
			exitOnError(fmt.Errorf("Error while validating edge rule: %s", err))
		}

		if err = client.Create("edgerule", &edgeRule, nil); err != nil {
			exitOnError(err)
		}

		printJSON(edgeRule)
	},
}

// EdgeRuleGet skydive edge get command
var EdgeRuleGet = &cobra.Command{
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
		var edge api.EdgeRule
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}
		if err := client.Get("edgerule", args[0], &edge); err != nil {
			exitOnError(err)
		}
		printJSON(&edge)
	},
}

// EdgeRuleList skydive edge list command
var EdgeRuleList = &cobra.Command{
	Use:          "list",
	Short:        "list",
	Long:         "list",
	SilenceUsage: false,

	Run: func(cmd *cobra.Command, args []string) {
		var edges map[string]api.EdgeRule
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.List("edgerule", &edges); err != nil {
			exitOnError(err)
		}
		printJSON(edges)
	},
}

// EdgeRuleDelete skydive edge delete command
var EdgeRuleDelete = &cobra.Command{
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
			exitOnError(err)
		}

		for _, id := range args {
			if err := client.Delete("edgerule", id); err != nil {
				logging.GetLogger().Error(err.Error())
			}
		}
	},
}

func addCreateEdgeRuleFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&name, "name", "", "", "rule name")
	cmd.Flags().StringVarP(&description, "description", "", "", "rule description")
	cmd.Flags().StringVarP(&src, "src", "", "", "src node gremlin expression")
	cmd.Flags().StringVarP(&dst, "dst", "", "", "dst node gremlin expression")
	cmd.Flags().StringVarP(&relationType, "relationtype", "", "", "relation type of the link")
	cmd.Flags().StringVarP(&metadata, "metadata", "", "", "edge metadata")
}

func init() {
	EdgeRuleCmd.AddCommand(EdgeRuleCreate)
	EdgeRuleCmd.AddCommand(EdgeRuleList)
	EdgeRuleCmd.AddCommand(EdgeRuleGet)
	EdgeRuleCmd.AddCommand(EdgeRuleDelete)

	addCreateEdgeRuleFlags(EdgeRuleCreate)
}
