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
	"encoding/json"
	"fmt"
	"os"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"
	"github.com/redhat-cip/skydive/topology/graph"

	"github.com/spf13/cobra"
)

var (
	alertName        string
	alertDescription string
	alertSelect      string
	alertTest        string
	alertAction      string
)

var Client = &cobra.Command{
	Use:          "client",
	Short:        "Skydive client",
	Long:         "Skydive client",
	SilenceUsage: true,
}

var Alert = &cobra.Command{
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
		alert := graph.AlertTest{}
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
		var alerts map[string]graph.AlertTest
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
		var alert graph.AlertTest
		client := rpc.NewClientFromConfig()
		if err := client.Get("alert", args[0], &alert); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
		printJSON(&alert)
	},
}

var AlertUpdate = &cobra.Command{
	Use:   "update [alert]",
	Short: "Update alert",
	Long:  "Update alert",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var alert graph.AlertTest
		client := rpc.NewClientFromConfig()
		client.Get("alert", args[0], &alert)
		setFromFlag(cmd, "name", &alert.Name)
		setFromFlag(cmd, "description", &alert.Description)
		setFromFlag(cmd, "select", &alert.Select)
		setFromFlag(cmd, "action", &alert.Action)
		setFromFlag(cmd, "test", &alert.Test)
		if err := client.Update("alert", alert.UUID.String(), &alert); err != nil {
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

func printJSON(obj interface{}) {
	s, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		logging.GetLogger().Errorf(err.Error())
		os.Exit(1)
	}
	fmt.Println(string(s))
}

func setFromFlag(cmd *cobra.Command, flag string, value *string) {
	if flag := cmd.LocalFlags().Lookup(flag); flag.Changed {
		*value = flag.Value.String()
	}
}

func addAlertFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&alertName, "name", "n", "", "alert name")
	cmd.Flags().StringVarP(&alertDescription, "description", "d", "", "alert description")
	cmd.Flags().StringVarP(&alertSelect, "select", "s", "", "alert select criteria")
	cmd.Flags().StringVarP(&alertTest, "test", "t", "", "alert test")
	cmd.Flags().StringVarP(&alertAction, "action", "a", "", "alert action")
}

func init() {
	Client.AddCommand(Alert)
	Alert.AddCommand(AlertList)
	Alert.AddCommand(AlertGet)
	Alert.AddCommand(AlertCreate)
	Alert.AddCommand(AlertUpdate)
	Alert.AddCommand(AlertDelete)

	addAlertFlags(AlertCreate)
	addAlertFlags(AlertUpdate)
}
