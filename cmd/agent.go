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
	"os"
	"os/signal"
	"syscall"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"

	"github.com/spf13/cobra"
)

var Agent = &cobra.Command{
	Use:          "agent",
	Short:        "Skydive agent",
	Long:         "Skydive agent",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		logging.SetLoggingID("agent")
		logging.GetLogger().Notice("Skydive Agent starting...")
		agent := agent.NewAgent()
		agent.Start()

		logging.GetLogger().Notice("Skydive Agent started")
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		agent.Stop()

		logging.GetLogger().Notice("Skydive Agent stopped.")
	},
}

func init() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	Agent.Flags().String("host-id", host, "ID used to reference the agent, defaults to hostname")
	config.GetConfig().BindPFlag("host_id", Agent.Flags().Lookup("host-id"))

	Agent.Flags().String("listen", "127.0.0.1:8081", "address and port for the agent API")
	config.GetConfig().BindPFlag("agent.listen", Agent.Flags().Lookup("listen"))

	Agent.Flags().String("ovsdb", "unix:///var/run/openvswitch/db.sock", "ovsdb connection")
	config.GetConfig().BindPFlag("ovs.ovsdb", Agent.Flags().Lookup("ovsdb"))
}
