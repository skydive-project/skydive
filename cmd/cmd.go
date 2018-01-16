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
	"fmt"
	"os"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/spf13/cobra"
)

var (
	// CfgFiles skydive configuration files
	CfgFiles   []string
	cfgBackend string
)

// LoadConfiguration from a configuration file
func LoadConfiguration() {
	if len(CfgFiles) != 0 {
		if err := config.InitConfig(cfgBackend, CfgFiles); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize config: %s", err.Error())
			os.Exit(1)
		}

		if err := logging.InitLogging(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize logging system: %s", err.Error())
			os.Exit(1)
		}
	}
}

// RootCmd skydive root command
var RootCmd = &cobra.Command{
	Use:          "skydive [sub]",
	Short:        "Skydive",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		LoadConfiguration()
	},
}

func init() {
	RootCmd.PersistentFlags().StringArrayVarP(&CfgFiles, "conf", "c", []string{}, "location of Skydive agent config files")
	RootCmd.PersistentFlags().StringVarP(&cfgBackend, "config-backend", "b", "file", "configuration backend (defaults to file)")
	RootCmd.AddCommand(VersionCmd)
	RootCmd.AddCommand(Client)
	RootCmd.AddCommand(BashCompletion)
}
