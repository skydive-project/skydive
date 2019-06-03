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
	"io/ioutil"
	"os"

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	srcNode          string
	dstNode          string
	srcIP            string
	srcMAC           string
	srcPort          uint16
	dstPort          uint16
	dstIP            string
	dstMAC           string
	packetType       string
	payload          string
	pcap             string
	id               uint64
	count            uint64
	interval         uint64
	increment        bool
	incrementPayload int64
	ttl              uint8
)

// PacketInjectorCmd skydive inject-packet root command
var PacketInjectorCmd = &cobra.Command{
	Use:          "inject-packet",
	Short:        "Inject packets",
	Long:         "Inject packets",
	SilenceUsage: false,
}

// PacketInjectionCreate describes the command to create a packet injection
var PacketInjectionCreate = &cobra.Command{
	Use:          "create",
	Short:        "create packet injection",
	Long:         "create packet injection",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			exitOnError(err)
		}

		var pcapContent []byte
		if pcap != "" {
			f, err := os.Open(pcap)
			if err != nil {
				exitOnError(err)
			}

			pcapContent, err = ioutil.ReadAll(f)
			if err != nil {
				exitOnError(err)
			}
		}

		packet := &api.PacketInjection{
			Src:              srcNode,
			Dst:              dstNode,
			SrcIP:            srcIP,
			SrcMAC:           srcMAC,
			SrcPort:          srcPort,
			DstIP:            dstIP,
			DstMAC:           dstMAC,
			DstPort:          dstPort,
			Type:             packetType,
			Payload:          payload,
			Pcap:             pcapContent,
			ICMPID:           uint16(id),
			Count:            count,
			Interval:         interval,
			Increment:        increment,
			IncrementPayload: incrementPayload,
			TTL:              ttl,
		}

		if err = validator.Validate(packet); err != nil {
			exitOnError(err)
		}

		if err := client.Create("injectpacket", &packet); err != nil {
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

func addInjectPacketFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&srcNode, "src", "", "", "source node gremlin expression (mandatory)")
	cmd.Flags().StringVarP(&dstNode, "dst", "", "", "destination node gremlin expression")
	cmd.Flags().StringVarP(&srcIP, "srcIP", "", "", "source node IP")
	cmd.Flags().StringVarP(&dstIP, "dstIP", "", "", "destination node IP")
	cmd.Flags().StringVarP(&srcMAC, "srcMAC", "", "", "source node MAC")
	cmd.Flags().StringVarP(&dstMAC, "dstMAC", "", "", "destination node MAC")
	cmd.Flags().Uint16VarP(&srcPort, "srcPort", "", 0, "source port for TCP packet")
	cmd.Flags().Uint16VarP(&dstPort, "dstPort", "", 0, "destination port for TCP packet")
	cmd.Flags().StringVarP(&packetType, "type", "", "icmp4", "packet type: icmp4, icmp6, tcp4, tcp6, udp4 and udp6")
	cmd.Flags().StringVarP(&payload, "payload", "", "", "payload")
	cmd.Flags().StringVar(&pcap, "pcap", "", "PCAP file")
	cmd.Flags().Uint64VarP(&id, "id", "", 0, "ICMP identification")
	cmd.Flags().BoolVarP(&increment, "increment", "", false, "increment ICMP id for each packet")
	cmd.Flags().Int64VarP(&incrementPayload, "incrementPayload", "", 0, "increase payload for each packet")
	cmd.Flags().Uint64VarP(&count, "count", "", 1, "number of packets to be generated")
	cmd.Flags().Uint64VarP(&interval, "interval", "", 1000, "wait interval milliseconds between sending each packet")
	cmd.Flags().Uint8VarP(&ttl, "ttl", "", 64, "time-to-live")
}

func init() {
	PacketInjectorCmd.AddCommand(PacketInjectionList)
	PacketInjectorCmd.AddCommand(PacketInjectionGet)
	PacketInjectorCmd.AddCommand(PacketInjectionDelete)
	PacketInjectorCmd.AddCommand(PacketInjectionCreate)

	addInjectPacketFlags(PacketInjectionCreate)
}
