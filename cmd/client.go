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

package cmd

import (
	"os"

	"github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/config"
	"github.com/spf13/cobra"
)

var analyzerAddr string

var Client = &cobra.Command{
	Use:          "client",
	Short:        "Skydive client",
	Long:         "Skydive client",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Root().PersistentPreRun(cmd.Root(), args)
		if analyzerAddr != "" {
			config.GetConfig().Set("agent.analyzers", analyzerAddr)
		} else {
			config.GetConfig().SetDefault("agent.analyzers", "localhost:8082")
		}
	},
}

func init() {
	Client.PersistentFlags().StringVarP(&client.AuthenticationOpts.Username, "username", "", os.Getenv("SKYDIVE_USERNAME"), "username auth parameter")
	Client.PersistentFlags().StringVarP(&client.AuthenticationOpts.Password, "password", "", os.Getenv("SKYDIVE_PASSWORD"), "password auth parameter")
	Client.PersistentFlags().StringVarP(&analyzerAddr, "analyzer", "", os.Getenv("SKYDIVE_ANALYZER"), "analyzer address")

	Client.AddCommand(client.AlertCmd)
	Client.AddCommand(client.CaptureCmd)
	Client.AddCommand(client.TopologyCmd)
	Client.AddCommand(client.ShellCmd)
	Client.AddCommand(client.PacketInjectorCmd)
}
