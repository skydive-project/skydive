/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/robertkrimen/otto"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/js"
	"github.com/skydive-project/skydive/logging"
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

	if err := validator.Validate(workflow); err != nil {
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
	PreRun: func(cmd *cobra.Command, args []string) {
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		workflow, err := loadWorklow(workflowPath)
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		if err := client.Create("workflow", &workflow); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
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
			logging.GetLogger().Critical(err)
			os.Exit(1)
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
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		if err := client.List("workflow", &workflows); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
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
		var workflow types.Workflow
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		if err := client.Get("workflow", args[0], &workflow); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		runtime, err := js.NewRuntime()
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		runtime.Start()
		runtime.RegisterAPIClient(client)

		result, err := runtime.Exec("(" + workflow.Source + ")")
		if err != nil {
			logging.GetLogger().Errorf("Error while compile workflow %s: %s", workflow.Source, result.String())
			os.Exit(1)
		}

		params := make([]interface{}, len(args)-1)
		for i, arg := range args[1:] {
			params[i] = arg
		}

		result, err = result.Call(result, params...)
		if err != nil {
			logging.GetLogger().Errorf("Error while executing workflow: %s", result.String())
			os.Exit(1)
		}

		if !result.IsObject() {
			logging.GetLogger().Errorf("Workflow is expected to return a promise, returned %s", result.Class())
			os.Exit(1)
		}

		done := make(chan otto.Value)
		promise := result.Object()

		finally, err := runtime.ToValue(func(call otto.FunctionCall) otto.Value {
			result := call.Argument(0)
			done <- result
			return result
		})

		result, _ = promise.Call("then", finally)
		promise = result.Object()
		promise.Call("catch", finally)

		result = <-done

		runtime.Set("result", result)
		runtime.Exec("console.log(JSON.stringify(result))")
	},
}

func init() {
	WorkflowCmd.AddCommand(WorkflowCreate)
	WorkflowCmd.AddCommand(WorkflowDelete)
	WorkflowCmd.AddCommand(WorkflowList)
	WorkflowCmd.AddCommand(WorkflowCall)

	WorkflowCreate.Flags().StringVarP(&workflowPath, "path", "", "", "Workflow path")
}
