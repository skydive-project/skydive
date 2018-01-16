// +build linux

/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/version"

	"github.com/spf13/cobra"
)

// Analyzer skydive analyzer root command
var Analyzer = &cobra.Command{
	Use:          "analyzer",
	Short:        "Skydive analyzer",
	Long:         "Skydive analyzer",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		config.GetConfig().Set("logging.id", "agent")
		logging.GetLogger().Noticef("Skydive Analyzer %s starting...", version.Version)

		server, err := analyzer.NewServerFromConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create analyzer: %s", err.Error())
			os.Exit(1)
		}

		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start analyzer: %s", err.Error())
			os.Exit(1)
		}

		logging.GetLogger().Notice("Skydive Analyzer started !")
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		server.Stop()

		logging.GetLogger().Notice("Skydive Analyzer stopped.")
	},
}

func init() {
	RootCmd.AddCommand(Analyzer)

	Analyzer.Flags().String("listen", "127.0.0.1:8082", "address and port for the analyzer API")
	config.GetConfig().BindPFlag("analyzer.listen", Analyzer.Flags().Lookup("listen"))
}
