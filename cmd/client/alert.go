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
	"os"

	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"

	"github.com/spf13/cobra"
)

var (
	alertName        string
	alertDescription string
	alertSelect      string
	alertTest        string
	alertAction      string
)

var AlertCmd = &cobra.Command{
	Use:          "alert",
	Short:        "Manage alerts",
	Long:         "Manage alerts",
	SilenceUsage: false,
}

var AlertCreate = &cobra.Command{
	Use:   "create",
	Short: "Create alert",
	Long:  "Create alert",
	Run: func(cmd *cobra.Command, args []string) {
		client := rpc.NewClientFromConfig()
		alert := api.NewAlert()
		setFromFlag(cmd, "name", &alert.Name)
		setFromFlag(cmd, "description", &alert.Description)
		setFromFlag(cmd, "select", &alert.Select)
		setFromFlag(cmd, "action", &alert.Action)
		setFromFlag(cmd, "test", &alert.Test)
		client.Create("alert", &alert)
		printJSON(&alert)
	},
}

var AlertList = &cobra.Command{
	Use:   "list",
	Short: "List alerts",
	Long:  "List alerts",
	Run: func(cmd *cobra.Command, args []string) {
		var alerts map[string]api.Alert
		client := rpc.NewClientFromConfig()
		if err := client.List("alert", &alerts); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
		printJSON(alerts)
	},
}

var AlertGet = &cobra.Command{
	Use:   "get [alert]",
	Short: "Display alert",
	Long:  "Display alert",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var alert api.Alert
		client := rpc.NewClientFromConfig()
		if err := client.Get("alert", args[0], &alert); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
		printJSON(&alert)
	},
}

var AlertDelete = &cobra.Command{
	Use:   "delete [alert]",
	Short: "Delete alert",
	Long:  "Delete alert",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client := rpc.NewClientFromConfig()
		if err := client.Delete("alert", args[0]); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
	},
}

func addAlertFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&alertName, "name", "n", "", "alert name")
	cmd.Flags().StringVarP(&alertDescription, "description", "d", "", "alert description")
	cmd.Flags().StringVarP(&alertSelect, "select", "s", "", "alert select criteria")
	cmd.Flags().StringVarP(&alertTest, "test", "t", "", "alert test")
	cmd.Flags().StringVarP(&alertAction, "action", "a", "", "alert action")
}

func init() {
	AlertCmd.AddCommand(AlertList)
	AlertCmd.AddCommand(AlertGet)
	AlertCmd.AddCommand(AlertCreate)
	AlertCmd.AddCommand(AlertDelete)

	addAlertFlags(AlertCreate)
}
