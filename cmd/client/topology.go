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
	"os"

	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/logging"
)

var gremlinQuery string

var TopologyCmd = &cobra.Command{
	Use:          "topology",
	Short:        "Request on topology",
	Long:         "Request on topology",
	SilenceUsage: false,
}

var TopologyRequest = &cobra.Command{
	Use:   "query",
	Short: "query topology",
	Long:  "query topology",
	Run: func(cmd *cobra.Command, args []string) {
		var value interface{}

		queryHelper := NewGremlinQueryHelper(&AuthenticationOpts)
		if err := queryHelper.Query(gremlinQuery, &value); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}

		printJSON(value)
	},
}

func addTopologyFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Gremlin Query")
}

func init() {
	TopologyCmd.AddCommand(TopologyRequest)
	TopologyRequest.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Gremlin Query")
}
