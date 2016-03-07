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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/tools"
)

func usage() {
	fmt.Printf("\nUsage: %s -trace <trace.pcap> [-pps <1000>] [-pktspersflow <5>] <sflow_agent[:port]>\n", filepath.Base(os.Args[0]))
}

func main() {
	pcaptrace := flag.String("trace", "", "PCAP trace file to read")
	pps := flag.Uint("pps", 1000, "Packets per second")
	pktsPerFlow := flag.Uint("pktspersflow", 5, "Number of Packets per SFlow Datagram")
	flag.CommandLine.Usage = usage
	flag.Parse()

	if *pcaptrace == "" {
		usage()
		os.Exit(1)
	}

	sflowAgent := strings.Split(os.Args[len(os.Args)-1], ":")
	addr := sflowAgent[0]
	port := 6345
	if len(sflowAgent) == 2 {
		var err error
		port, err = strconv.Atoi(sflowAgent[1])
		if err != nil {
			logging.GetLogger().Fatal("Can't parse UDP port ", err)
		}
	}

	err := tools.PCAP2SFlowReplay(addr, port, *pcaptrace, uint32(*pps), uint32(*pktsPerFlow))
	if err != nil {
		logging.GetLogger().Fatalf("Error during the replay: %s", err.Error())
	}
}
