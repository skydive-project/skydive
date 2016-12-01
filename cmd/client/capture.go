/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"fmt"
	"os"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	bpfFilter          string
	captureName        string
	captureDescription string
	captureType        string
)

var CaptureCmd = &cobra.Command{
	Use:          "capture",
	Short:        "Manage captures",
	Long:         "Manage captures",
	SilenceUsage: false,
}

var CaptureCreate = &cobra.Command{
	Use:   "create",
	Short: "Create capture",
	Long:  "Create capture",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Criticalf(err.Error())
		}

		capture := api.NewCapture(gremlinQuery, bpfFilter)
		capture.Name = captureName
		capture.Description = captureDescription
		capture.Type = captureType
		if err := validator.Validate(capture); err != nil {
			fmt.Println(err.Error())
			cmd.Usage()
			os.Exit(1)
		}
		if err := client.Create("capture", &capture); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
		printJSON(&capture)
	},
}

var CaptureList = &cobra.Command{
	Use:   "list",
	Short: "List captures",
	Long:  "List captures",
	Run: func(cmd *cobra.Command, args []string) {
		var captures map[string]api.Capture
		client, err := api.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Criticalf(err.Error())
		}

		if err := client.List("capture", &captures); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
		printJSON(captures)
	},
}

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
		client, err := api.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Criticalf(err.Error())
		}

		if err := client.Get("capture", args[0], &capture); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
		printJSON(&capture)
	},
}

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
		client, err := api.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Criticalf(err.Error())
		}

		if err := client.Delete("capture", args[0]); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
	},
}

func addCaptureFlags(cmd *cobra.Command) {
	types := []string{}
	found := map[string]bool{}
	for _, v := range common.CaptureTypes {
		for _, t := range v.Allowed {
			if found[t] != true {
				found[t] = true
				types = append(types, t)
			}
		}
	}
	helpText := fmt.Sprintf("Allowed capture types: %v", types)
	cmd.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Gremlin Query")
	cmd.Flags().StringVarP(&bpfFilter, "bpf", "", "", "BPF filter")
	cmd.Flags().StringVarP(&captureName, "name", "", "", "capture name")
	cmd.Flags().StringVarP(&captureDescription, "description", "", "", "capture description")
	cmd.Flags().StringVarP(&captureType, "type", "", "", helpText)
}

func init() {
	CaptureCmd.AddCommand(CaptureList)
	CaptureCmd.AddCommand(CaptureCreate)
	CaptureCmd.AddCommand(CaptureGet)
	CaptureCmd.AddCommand(CaptureDelete)

	addCaptureFlags(CaptureCreate)
}
