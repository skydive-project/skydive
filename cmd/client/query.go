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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/graffiti/logging"
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
		queryHelper := client.NewGremlinQueryHelper(&AuthenticationOpts)

		switch outputFormat {
		case "json":
			data, err := queryHelper.Query(gremlinQuery)
			if err != nil {
				exitOnError(err)
			}

			var out bytes.Buffer
			json.Indent(&out, data, "", "\t")
			out.WriteTo(os.Stdout)
		case "dot":
			header := make(http.Header)
			header.Set("Accept", "text/vnd.graphviz")
			resp, err := queryHelper.Request(gremlinQuery, header)
			if err != nil {
				exitOnError(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				data, _ := ioutil.ReadAll(resp.Body)
				exitOnError(fmt.Errorf("%s: %s", resp.Status, string(data)))
			}
			bufio.NewReader(resp.Body).WriteTo(os.Stdout)
		case "pcap":
			header := make(http.Header)
			header.Set("Accept", "application/vnd.tcpdump.pcap")
			resp, err := queryHelper.Request(gremlinQuery, header)
			if err != nil {
				exitOnError(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				data, _ := ioutil.ReadAll(resp.Body)
				exitOnError(fmt.Errorf("%s: %s", resp.Status, string(data)))
			}

			bufio.NewReader(resp.Body).WriteTo(os.Stdout)
		default:
			logging.GetLogger().Errorf("Invalid output format %s", outputFormat)
			os.Exit(1)
		}
	},
}

func init() {
	QueryCmd.Flags().StringVarP(&outputFormat, "format", "", "json", "Output format (json, dot or pcap)")
}
