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

package skydive

import (
	"os"
	"path"
	"strings"

	"github.com/skydive-project/skydive/cmd"
	"github.com/skydive-project/skydive/cmd/agent"
	"github.com/skydive-project/skydive/cmd/allinone"
	"github.com/skydive-project/skydive/cmd/analyzer"
	"github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/cmd/completion"
	"github.com/skydive-project/skydive/cmd/config"
	"github.com/skydive-project/skydive/cmd/injector"
	"github.com/skydive-project/skydive/cmd/seed"
	"github.com/skydive-project/skydive/cmd/version"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/spf13/cobra"
)

// RootCmd describes the skydive root command
var RootCmd = &cobra.Command{
	Use:          "skydive [sub]",
	Short:        "Skydive",
	SilenceUsage: true,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		if verbose, _ := c.Flags().GetBool("verbose"); verbose {
			os.Setenv("SKYDIVE_LOGGING_LEVEL", "debug")
		}

		if err := config.LoadConfiguration(cmd.CfgBackend, cmd.CfgFiles); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.PersistentFlags().StringArrayVarP(&cmd.CfgFiles, "conf", "c", []string{}, "location of Skydive configuration files, default try loading /etc/skydive/skydive.yml if exist")
	RootCmd.PersistentFlags().StringVarP(&cmd.CfgBackend, "config-backend", "b", "file", "configuration backend (defaults to file)")
	RootCmd.PersistentFlags().BoolP("verbose", "", false, "verbose mode")

	executable := path.Base(os.Args[0])
	if strings.TrimSuffix(executable, path.Ext(executable)) == "skydive-cli" {
		RootCmd.Use = "skydive-cli"
		RootCmd.Short = "Skydive client"
		RootCmd.AddCommand(completion.BashCompletion)
		RootCmd.AddCommand(version.VersionCmd)
		client.RegisterClientCommands(RootCmd)
	} else {
		RootCmd.AddCommand(agent.AgentCmd)
		RootCmd.AddCommand(analyzer.AnalyzerCmd)
		RootCmd.AddCommand(completion.BashCompletion)
		RootCmd.AddCommand(client.ClientCmd)
		RootCmd.AddCommand(seed.SeedCmd)
		RootCmd.AddCommand(version.VersionCmd)
		RootCmd.AddCommand(injector.InjectPacketCmd)

		if allinone.AllInOneCmd != nil {
			RootCmd.AddCommand(allinone.AllInOneCmd)
		}
	}
}
