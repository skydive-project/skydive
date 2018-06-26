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
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	gremlinQuery string
	outputFormat string
)

// topologyCmd skydive topology root command
var topologyCmd = &cobra.Command{
	Use:          "topology",
	Short:        "Request on topology [deprecated: use 'client query' instead]",
	Long:         "Request on topology [deprecated: use 'client query' instead]",
	SilenceUsage: false,
}

var topologyRequest = &cobra.Command{
	Use:   "query",
	Short: "query topology",
	Long:  "query topology",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "The 'client topology query' command is deprecated. Please use 'client query' instead")
		queryCmd.Run(cmd, []string{gremlinQuery})
	},
}

func init() {
	topologyCmd.AddCommand(topologyRequest)
	topologyRequest.Flags().StringVarP(&gremlinQuery, "gremlin", "", "G", "Gremlin Query")
	topologyRequest.Flags().StringVarP(&outputFormat, "format", "", "json", "Output format (json, dot or pcap)")
}
