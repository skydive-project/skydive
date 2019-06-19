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
	"os"
	"time"

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/cmd/injector"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	srcNode string
	dstNode string
)

// PacketInjectorCmd skydive inject-packet root command
var PacketInjectorCmd = &cobra.Command{
	Use:          "injection",
	Short:        "Inject packets",
	Long:         "Inject packets",
	Aliases:      []string{"inject-packet"},
	SilenceUsage: false,
}

// PacketInjectionCreate describes the command to create a packet injection
var PacketInjectionCreate = &cobra.Command{
	Use:          "create",
	Short:        "create packet injection",
	Long:         "create packet injection",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		crudClient, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		request, err := injector.GetPacketInjectRequest()
		if err != nil {
			exitOnError(err)
		}

		packet := &api.PacketInjection{
			Src:              srcNode,
			Dst:              dstNode,
			SrcPort:          request.SrcPort,
			DstPort:          request.DstPort,
			Type:             request.Type,
			Payload:          request.Payload,
			Pcap:             request.Pcap,
			ICMPID:           request.ICMPID,
			Count:            request.Count,
			Interval:         request.Interval,
			Increment:        request.Increment,
			IncrementPayload: request.IncrementPayload,
			TTL:              request.TTL,
		}

		if request.SrcIP != nil {
			packet.SrcIP = request.SrcIP.String()
		}

		if request.DstIP != nil {
			packet.DstIP = request.DstIP.String()
		}

		if request.SrcMAC != nil {
			packet.SrcMAC = request.SrcMAC.String()
		}

		if request.DstMAC != nil {
			packet.DstMAC = request.DstMAC.String()
		}

		if err = validator.Validate(packet); err != nil {
			exitOnError(err)
		}

		var createOpts *http.CreateOptions
		if packet.Pcap == nil {
			ttl := 5 * time.Second
			if packet.Interval != 0 {
				ttl = time.Duration(packet.Interval*packet.Count)*time.Millisecond + 5*time.Second
			}
			createOpts = &http.CreateOptions{TTL: ttl}
		}

		if err := crudClient.Create("injectpacket", &packet, createOpts); err != nil {
			exitOnError(err)
		}

		printJSON(packet)
	},
}

// PacketInjectionGet describes the command to retrieve a packet injection
var PacketInjectionGet = &cobra.Command{
	Use:   "get",
	Short: "get packet injection",
	Long:  "get packet injection",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var injection api.PacketInjection
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.Get("injectpacket", args[0], &injection); err != nil {
			exitOnError(err)
		}
		printJSON(&injection)
	},
}

// PacketInjectionList describes the command to list all the packet injections
var PacketInjectionList = &cobra.Command{
	Use:   "list",
	Short: "list packet injections",
	Long:  "list packet injections",
	Run: func(cmd *cobra.Command, args []string) {
		var injections map[string]api.PacketInjection
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		if err := client.List("injectpacket", &injections); err != nil {
			exitOnError(err)
		}
		printJSON(injections)
	},
}

// PacketInjectionDelete describes the command to delete a packet injection
var PacketInjectionDelete = &cobra.Command{
	Use:   "delete [injection]",
	Short: "Delete injection",
	Long:  "Delete packet injection",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		for _, id := range args {
			if err := client.Delete("injectpacket", id); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

func init() {
	PacketInjectorCmd.AddCommand(PacketInjectionList)
	PacketInjectorCmd.AddCommand(PacketInjectionGet)
	PacketInjectorCmd.AddCommand(PacketInjectionDelete)
	PacketInjectorCmd.AddCommand(PacketInjectionCreate)

	PacketInjectionCreate.Flags().StringVarP(&srcNode, "src", "", "", "source node gremlin expression (mandatory)")
	PacketInjectionCreate.Flags().StringVarP(&dstNode, "dst", "", "", "destination node gremlin expression")
	injector.AddInjectPacketInjectFlags(PacketInjectionCreate)
}
