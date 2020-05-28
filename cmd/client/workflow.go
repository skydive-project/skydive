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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	workflowPath string
)

// WorkflowCmd describe the "workflow" root command
var WorkflowCmd = &cobra.Command{
	Use:          "workflow",
	Short:        "Manage workflows",
	Long:         "Manage workflows",
	SilenceUsage: false,
}

func loadWorklow(path string) (*types.Workflow, error) {
	file, err := os.Open(workflowPath)
	if err != nil {
		return nil, err
	}

	yml, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	workflow := &types.Workflow{}
	if err := yaml.Unmarshal([]byte(yml), &workflow); err != nil {
		return nil, err
	}

	if err := validator.Validate("workflow", workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

// WorkflowCreate describes the "workflow create" command
var WorkflowCreate = &cobra.Command{
	Use:          "create",
	Short:        "create workflow",
	Long:         "create workflow",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		workflow, err := loadWorklow(workflowPath)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Create("workflow", &workflow, nil); err != nil {
			exitOnError(err)
		}
		printJSON(workflow)
	},
}

// WorkflowDelete describes the "workflow delete" command
var WorkflowDelete = &cobra.Command{
	Use:          "delete",
	Short:        "delete workflow",
	Long:         "delete workflow",
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
			if err := client.Delete("workflow", id); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

// WorkflowList describes the "workflow list" command
var WorkflowList = &cobra.Command{
	Use:          "list",
	Short:        "List workflows",
	Long:         "List workflows",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		var workflows map[string]types.Workflow
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.List("workflow", &workflows); err != nil {
			exitOnError(err)
		}
		printJSON(workflows)
	},
}

// WorkflowCall describes the "workflow call" command
var WorkflowCall = &cobra.Command{
	Use:          "call workflow",
	Short:        "Call workflow",
	Long:         "Call workflow",
	SilenceUsage: false,
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var workflowCall types.WorkflowCall
		var result interface{}
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		params := make([]interface{}, len(args)-1)
		for i, arg := range args[1:] {
			params[i] = arg
		}

		workflowCall.Params = params

		s, err := json.Marshal(workflowCall)
		if err != nil {
			exitOnError(err)
		}

		contentReader := bytes.NewReader(s)
		resp, err := client.Request("POST", "workflow/"+args[0]+"/call", contentReader, nil)
		if err != nil {
			exitOnError(err)
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&result); err != nil {
			exitOnError(err)
		}

		printJSON(result)
	},
}

func init() {
	WorkflowCmd.AddCommand(WorkflowCreate)
	WorkflowCmd.AddCommand(WorkflowDelete)
	WorkflowCmd.AddCommand(WorkflowList)
	WorkflowCmd.AddCommand(WorkflowCall)

	WorkflowCreate.Flags().StringVarP(&workflowPath, "path", "", "", "Workflow path")
}
