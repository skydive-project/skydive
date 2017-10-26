/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"os"

	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/logging"
)

// QueryCmd skydive topology query command
var QueryCmd = &cobra.Command{
	Use:   "query [gremlin]",
	Short: "Issue Gremlin queries",
	Long:  "Issue Gremlin queries",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 || args[0] == "" {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		gremlinQuery = args[0]
		queryHelper := NewGremlinQueryHelper(&AuthenticationOpts)

		switch outputFormat {
		case "json":
			var value interface{}
			if err := queryHelper.Query(gremlinQuery, &value); err != nil {
				logging.GetLogger().Error(err.Error())
				os.Exit(1)
			}
			printJSON(value)
		case "dot":
			header := make(http.Header)
			header.Set("Accept", "vnd.graphviz")
			resp, err := queryHelper.Request(gremlinQuery, header)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				os.Exit(1)
			}
			defer resp.Body.Close()
			data, _ := ioutil.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				logging.GetLogger().Errorf("%s: %s", resp.Status, string(data))
				os.Exit(1)
			}
			fmt.Println(string(data))
		case "pcap":
			header := make(http.Header)
			header.Set("Accept", "vnd.tcpdump.pcap")
			resp, err := queryHelper.Request(gremlinQuery, header)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				os.Exit(1)
			}
			defer resp.Body.Close()
			data, _ := ioutil.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				logging.GetLogger().Errorf("%s: %s", resp.Status, string(data))
				os.Exit(1)
			}
			fmt.Print(string(data))
		default:
			logging.GetLogger().Errorf("Invalid output format %s", outputFormat)
			os.Exit(1)
		}
	},
}

func init() {
	QueryCmd.Flags().StringVarP(&outputFormat, "format", "", "json", "Output format (json, dot or pcap)")
}
