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

	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/spf13/cobra"
)

var (
	authenticationOpts shttp.AuthenticationOpts
	gremlinQuery       string
	analyzerAddr       string
)

var Client = &cobra.Command{
	Use:          "client",
	Short:        "Skydive client",
	Long:         "Skydive client",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if flag := cmd.Flags().Lookup("analyzer"); !flag.Changed {
			config.GetConfig().SetDefault("agent.analyzers", "localhost:8082")
		} else {
			config.GetConfig().Set("agent.analyzers", flag.Value)
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

func init() {
	Client.PersistentFlags().StringVarP(&authenticationOpts.Username, "username", "", os.Getenv("SKYDIVE_USERNAME"), "username auth parameter")
	Client.PersistentFlags().StringVarP(&authenticationOpts.Password, "password", "", os.Getenv("SKYDIVE_PASSWORD"), "password auth parameter")
	Client.PersistentFlags().String("analyzer", "", "analyzer address")

	Client.AddCommand(AlertCmd)
	Client.AddCommand(CaptureCmd)
	Client.AddCommand(TopologyCmd)
	Client.AddCommand(ShellCmd)
	Client.AddCommand(PacketInjectorCmd)
}
