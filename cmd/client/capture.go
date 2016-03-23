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

	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"

	"github.com/spf13/cobra"
)

var (
	probePath string
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
		if len(probePath) == 0 {
			fmt.Println("You need to specify a probe path")
			cmd.Usage()
			os.Exit(1)
		}

		client := rpc.NewClientFromConfig()
		capture := api.NewCapture(probePath)
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
		client := rpc.NewClientFromConfig()
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
		client := rpc.NewClientFromConfig()
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
		client := rpc.NewClientFromConfig()
		if err := client.Delete("capture", args[0]); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
	},
}

func addCaptureFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&probePath, "probepath", "p", "", "probepath")
}

func init() {
	CaptureCmd.AddCommand(CaptureList)
	CaptureCmd.AddCommand(CaptureCreate)
	CaptureCmd.AddCommand(CaptureGet)
	CaptureCmd.AddCommand(CaptureDelete)

	addCaptureFlags(CaptureCreate)
}
