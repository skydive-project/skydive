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

package main

import (
	"fmt"
	"os"

	"github.com/redhat-cip/skydive/cmd/agent"
	"github.com/redhat-cip/skydive/cmd/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/version"

	"github.com/spf13/cobra"
)

var showVersion bool
var cfgFile string

var rootCmd = &cobra.Command{
	Use:          "skydive [sub]",
	Short:        "Skydive",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cfgFile != "" {
			err := config.InitConfigFromFile(cfgFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Skydive",
	Run: func(cmd *cobra.Command, args []string) {
		version.PrintVersion()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "conf", "c", "", "location of Skydive agent config file")

	rootCmd.Flags().Int("ws-pong-timeout", 50, "WebSocket Ping/Pong timeout in second")
	config.GetConfig().BindPFlag("ws_pong_timeout", rootCmd.Flags().Lookup("ws-pong-timeout"))
}

func main() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(agent.Agent)
	rootCmd.AddCommand(analyzer.Analyzer)
	rootCmd.Execute()
}
