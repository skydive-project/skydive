/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"os"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	alertName        string
	alertDescription string
	alertExpression  string
	alertAction      string
	alertTrigger     string
)

// AlertCmd skydive alert root command
var AlertCmd = &cobra.Command{
	Use:          "alert",
	Short:        "Manage alerts",
	Long:         "Manage alerts",
	SilenceUsage: false,
}

// AlertCreate skydive alert creates command
var AlertCreate = &cobra.Command{
	Use:   "create",
	Short: "Create alert",
	Long:  "Create alert",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		alert := types.NewAlert()
		alert.Name = alertName
		alert.Description = alertDescription
		alert.Expression = alertExpression
		alert.Trigger = alertTrigger
		alert.Action = alertAction

		if err := validator.Validate("alert", alert); err != nil {
			exitOnError(err)
		}

		if err := client.Create("alert", &alert, nil); err != nil {
			exitOnError(err)
		}
		printJSON(&alert)
	},
}

// AlertList skydive alert list command
var AlertList = &cobra.Command{
	Use:   "list",
	Short: "List alerts",
	Long:  "List alerts",
	Run: func(cmd *cobra.Command, args []string) {
		var alerts map[string]types.Alert
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}
		if err := client.List("alert", &alerts); err != nil {
			exitOnError(err)
		}
		printJSON(alerts)
	},
}

// AlertGet skydive alert get command
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
		var alert types.Alert
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Get("alert", args[0], &alert); err != nil {
			exitOnError(err)
		}
		printJSON(&alert)
	},
}

// AlertDelete skydive alert delete command
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
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		for _, id := range args {
			if err := client.Delete("alert", id); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

func addAlertFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&alertName, "name", "", "", "alert name")
	cmd.Flags().StringVarP(&alertDescription, "description", "", "", "description of the alert")
	cmd.Flags().StringVarP(&alertTrigger, "trigger", "", "graph", "event that triggers the alert evaluation")
	cmd.Flags().StringVarP(&alertExpression, "expression", "", "", "Gremlin or JavaScript expression evaluated to trigger the alarm")
	cmd.Flags().StringVarP(&alertAction, "action", "", "", "can be either an empty string, or a URL (use 'file://' for local scripts)")
}

func init() {
	AlertCmd.AddCommand(AlertList)
	AlertCmd.AddCommand(AlertGet)
	AlertCmd.AddCommand(AlertCreate)
	AlertCmd.AddCommand(AlertDelete)

	addAlertFlags(AlertCreate)
}
