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
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/logging"
)

var (
	gremlinQuery string
	outputFormat string
)

// TopologyCmd skydive topology root command
var TopologyCmd = &cobra.Command{
	Use:          "topology",
	Short:        "Request on topology",
	Long:         "Request on topology",
	SilenceUsage: false,
}

// TopologyRequest skydive topology query command
var TopologyRequest = &cobra.Command{
	Use:   "query",
	Short: "query topology",
	Long:  "query topology",
	Run: func(cmd *cobra.Command, args []string) {
		queryHelper := NewGremlinQueryHelper(&AuthenticationOpts)

		switch outputFormat {
		case "json":
			var value interface{}
			if err := queryHelper.Query(gremlinQuery, &value); err != nil {
				logging.GetLogger().Fatalf(err.Error())
			}
			printJSON(value)
		case "dot":
			header := make(http.Header)
			header.Set("Accept", "vnd.graphviz")
			resp, err := queryHelper.Request(gremlinQuery, header)
			if err != nil {
				logging.GetLogger().Fatalf(err.Error())
			}
			defer resp.Body.Close()
			data, _ := ioutil.ReadAll(resp.Body)
			fmt.Println(string(data))
		default:
			logging.GetLogger().Fatalf("Invalid output format %s", outputFormat)
		}
	},
}

func addTopologyFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Gremlin Query")
}

func init() {
	TopologyCmd.AddCommand(TopologyRequest)
	TopologyRequest.Flags().StringVarP(&gremlinQuery, "gremlin", "", "G", "Gremlin Query")
	TopologyRequest.Flags().StringVarP(&outputFormat, "format", "", "json", "Output format (json or dot)")
}
