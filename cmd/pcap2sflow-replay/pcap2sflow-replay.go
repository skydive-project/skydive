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

package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/tools"
	"github.com/spf13/cobra"
)

var (
	pcapTrace   string
	pps         uint32
	pktsPerFlow uint32
	sflowAddr   string
	sflowPort   int
)

var replayCmd = &cobra.Command{
	Use:          "pcap2sflow-replay",
	Short:        "",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if pcapTrace == "" {
			cmd.Usage()
			os.Exit(1)
		}
		sflowFlag := cmd.LocalFlags().Lookup("sflow-agent").Value.String()
		sflowTuple := strings.Split(sflowFlag, ":")
		sflowAddr = sflowTuple[0]
		sflowPort = 6345
		if len(sflowTuple) == 2 {
			if port, err := strconv.Atoi(sflowTuple[1]); err != nil {
				logging.GetLogger().Fatal("Can't parse UDP port: ", err)
			} else {
				sflowPort = port
			}
		} else if len(sflowTuple) > 2 {
			logging.GetLogger().Fatal("Invalid format for sFlow agent address:", sflowFlag)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := tools.PCAP2SFlowReplay(sflowAddr, sflowPort, pcapTrace, pps, pktsPerFlow)
		if err != nil {
			logging.GetLogger().Fatalf("Error during the replay: %s", err.Error())
		}
	},
}

func main() {
	replayCmd.Flags().StringVarP(&pcapTrace, "trace", "t", "trace.pcap", "PCAP trace file to read")
	replayCmd.Flags().StringP("sflow-agent", "s", "localhost:6345", "sFlow agent address (addr[:port])")
	replayCmd.Flags().Uint32Var(&pps, "pps", 1000, "packets per second")
	replayCmd.Flags().Uint32Var(&pktsPerFlow, "pktspersflow", 5, "number of packets per sFlow datagram")
	replayCmd.Execute()
}
