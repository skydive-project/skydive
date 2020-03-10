/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/graffiti/logging"

	"github.com/spf13/cobra"
)

var (
	pcapTrace string
)

// PcapCmd skydive pcap root command
var PcapCmd = &cobra.Command{
	Use:   "pcap",
	Short: "Import flows from PCAP file",
	Long:  "Import flows from PCAP file",
	PreRun: func(cmd *cobra.Command, args []string) {
		if pcapTrace == "" {
			logging.GetLogger().Error("You need to specify a PCAP file")
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		file, err := os.Open(pcapTrace)
		if err != nil {
			exitOnError(err)
		}
		defer file.Close()

		resp, err := client.Request("POST", "pcap", file, nil)
		if err != nil {
			exitOnError(err)
		}

		if resp.StatusCode == http.StatusAccepted {
			fmt.Printf("%s was successfully imported\n", pcapTrace)
		} else {
			content, _ := ioutil.ReadAll(resp.Body)
			exitOnError(fmt.Errorf("Failed to import %s: %s", pcapTrace, string(content)))
		}
	},
}

func init() {
	PcapCmd.Flags().StringVarP(&pcapTrace, "trace", "t", "", "PCAP trace file to read")
}
