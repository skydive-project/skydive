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
	"os"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	srcNode    string
	dstNode    string
	packetType string
	payload    string
	count      int
)

var PacketInjectorCmd = &cobra.Command{
	Use:          "inject-packet",
	Short:        "inject packets",
	Long:         "inject packets",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Criticalf(err.Error())
		}

		packet := &api.PacketParamsReq{}
		packet.Src = srcNode
		packet.Dst = dstNode
		packet.Type = packetType
		packet.Payload = payload
		packet.Count = count
		if errs := validator.Validate(packet); errs != nil {
			fmt.Println("Error: ", errs)
			cmd.Usage()
			os.Exit(1)
		}
		if err := client.Create("injectpacket", &packet); err != nil {
			logging.GetLogger().Errorf(err.Error())
			os.Exit(1)
		}
	},
}

func addInjectPacketFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&srcNode, "src", "", "", "source node gremlin expression")
	cmd.Flags().StringVarP(&dstNode, "dst", "", "", "destination node gremlin expression")
	cmd.Flags().StringVarP(&packetType, "type", "", "icmp", "packet type: icmp")
	cmd.Flags().StringVarP(&payload, "payload", "", "", "payload")
	cmd.Flags().IntVarP(&count, "count", "", 1, "number of packets to be generated")
}

func init() {
	addInjectPacketFlags(PacketInjectorCmd)
}
