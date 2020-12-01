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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/config"
	gclient "github.com/skydive-project/skydive/graffiti/cmd/client"
	"github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
)

var (
	host         string
	analyzerAddr string

	// ClientCmd describes the skydive client root command
	ClientCmd = gclient.ClientCmd
	// CrudClient holds the client API CRUD client
	CrudClient *http.CrudClient
)

func exitOnError(err error) {
	logging.GetLogger().Error(err)
	os.Exit(1)
}

func printJSON(obj interface{}) {
	s, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		logging.GetLogger().Error(err)
		os.Exit(1)
	}
	fmt.Println(string(s))
}

// RegisterClientCommands registers the 'client' CLI subcommands
func RegisterClientCommands(cmd *cobra.Command) {
	cmd.AddCommand(AlertCmd)
	cmd.AddCommand(CaptureCmd)
	cmd.AddCommand(EdgeRuleCmd)
	cmd.AddCommand(NodeRuleCmd)
	cmd.AddCommand(PacketInjectorCmd)
	cmd.AddCommand(PcapCmd)
	cmd.AddCommand(ShellCmd)
	cmd.AddCommand(StatusCmd)
	cmd.AddCommand(WorkflowCmd)
}

func init() {
	ClientCmd.Short = "Skydive client"
	ClientCmd.Long = "Skydive client"

	persistentPreRun := ClientCmd.PersistentPreRun
	ClientCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.Root().PersistentPreRun(cmd.Root(), args)
		if analyzerAddr == "" {
			analyzerAddr = "127.0.0.1:8082"
		}

		gclient.HubAddress = analyzerAddr
		persistentPreRun(cmd, args)
		CrudClient = gclient.CrudClient
	}

	gclient.AuthenticationOpts.Username = os.Getenv("SKYDIVE_USERNAME")
	gclient.AuthenticationOpts.Password = os.Getenv("SKYDIVE_PASSWORD")

	ClientCmd.PersistentFlags().StringVarP(&analyzerAddr, "analyzer", "", os.Getenv("SKYDIVE_ANALYZER"), "analyzer address")

	config.BindPFlag("tls.ca_cert", ClientCmd.PersistentFlags().Lookup("cacert"))
	config.BindPFlag("tls.client_cert", ClientCmd.PersistentFlags().Lookup("cert"))
	config.BindPFlag("tls.client_key", ClientCmd.PersistentFlags().Lookup("key"))

	RegisterClientCommands(ClientCmd)
}
