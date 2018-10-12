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
	"io/ioutil"
	"net/http"
	"os"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"

	"github.com/spf13/cobra"
)

// StatusCmd implents the skydive 'status' command that
// return the status of an analyzer by quering its API
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show analyzer status",
	Long:  "Show analyzer status",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}

		resp, err := client.Request("GET", "status", nil, nil)
		if err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := ioutil.ReadAll(resp.Body)
			logging.GetLogger().Errorf("Failed to get status, %s: %s", resp.Status, data)
			os.Exit(1)
		}

		var status analyzer.Status
		if err := common.JSONDecode(resp.Body, &status); err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}

		printJSON(&status)
	},
}
