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

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/spf13/cobra"
)

var analyzerAddr string

// ClientCmd describe the skydive client root command
var ClientCmd = &cobra.Command{
	Use:          "client",
	Short:        "Skydive client",
	Long:         "Skydive client",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Root().PersistentPreRun(cmd.Root(), args)
		if analyzerAddr != "" {
			config.Set("analyzers", analyzerAddr)
		} else {
			config.SetDefault("analyzers", []string{"localhost:8082"})
		}
	},
}

// RegisterClientCommands registers the 'client' CLI subcommands
func RegisterClientCommands(cmd *cobra.Command) {
	cmd.AddCommand(AlertCmd)
	cmd.AddCommand(CaptureCmd)
	cmd.AddCommand(PacketInjectorCmd)
	cmd.AddCommand(PcapCmd)
	cmd.AddCommand(QueryCmd)
	cmd.AddCommand(ShellCmd)
	cmd.AddCommand(StatusCmd)
	cmd.AddCommand(TopologyCmd)
	cmd.AddCommand(WorkflowCmd)
	cmd.AddCommand(NodeRuleCmd)
	cmd.AddCommand(EdgeRuleCmd)
}

func exitOnError(err error) {
	logging.GetLogger().Error(err)
	os.Exit(1)
}

func init() {
	ClientCmd.PersistentFlags().StringVarP(&AuthenticationOpts.Username, "username", "", os.Getenv("SKYDIVE_USERNAME"), "username auth parameter")
	ClientCmd.PersistentFlags().StringVarP(&AuthenticationOpts.Password, "password", "", os.Getenv("SKYDIVE_PASSWORD"), "password auth parameter")
	ClientCmd.PersistentFlags().StringVarP(&analyzerAddr, "analyzer", "", os.Getenv("SKYDIVE_ANALYZER"), "analyzer address")

	RegisterClientCommands(ClientCmd)
}
