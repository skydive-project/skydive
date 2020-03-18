/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"io/ioutil"
	"net/http"

	"github.com/skydive-project/skydive/api/client"

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
			exitOnError(err)
		}

		resp, err := client.Request("GET", "status", nil, nil)
		if err != nil {
			exitOnError(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := ioutil.ReadAll(resp.Body)
			exitOnError(fmt.Errorf("Failed to get status, %s: %s", resp.Status, data))
		}

		var status interface{}
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&status); err != nil {
			exitOnError(err)
		}

		printJSON(&status)
	},
}
