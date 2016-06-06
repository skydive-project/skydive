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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/redhat-cip/skydive/api"
	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/logging"
)

var (
	gremlinQuery string
)

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
		client := shttp.NewRestClientFromConfig(&authenticationOpts)

		gq := api.Topology{GremlinQuery: gremlinQuery}
		s, err := json.Marshal(gq)
		if err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}

		contentReader := bytes.NewReader(s)

		resp, err := client.Request("GET", "api/topology", contentReader)
		if err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}

		if resp.StatusCode != 200 {
			data, _ := ioutil.ReadAll(resp.Body)
			logging.GetLogger().Errorf("%s: %s", resp.Status, string(data))
			os.Exit(1)
		}

		var values interface{}
		err = json.NewDecoder(resp.Body).Decode(&values)
		if err != nil {
			logging.GetLogger().Errorf("Unable to decode response: %s", err.Error())
			os.Exit(1)
		}

		printJSON(&values)
	},
}

func addTopologyFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Gremlin Query")
}

func init() {
	TopologyCmd.AddCommand(TopologyRequest)

	addTopologyFlags(TopologyRequest)
}
