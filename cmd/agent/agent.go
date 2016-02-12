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

package agent

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-cip/skydive/agent"
	"github.com/redhat-cip/skydive/logging"

	"github.com/spf13/cobra"
)

var Agent = &cobra.Command{
	Use:          "agent",
	Short:        "Skydive agent",
	Long:         "Skydive agent",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		logging.GetLogger().Notice("Skydive Agent starting...")
		agent := agent.NewAgent()
		agent.Start()

		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		agent.Stop()

		logging.GetLogger().Notice("Skydive Agent stopped.")
	},
}
