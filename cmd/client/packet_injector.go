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
	"net"
	"os"
	"time"

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/packetinjector"
	pi "github.com/skydive-project/skydive/packetinjector"
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
	id               uint16
	count            uint64
	interval         uint64
	mode             string
	incrementPayload int64
	ttl              uint8
)

// AddInjectPacketInjectFlags add the command line flags for a packet injection
func AddInjectPacketInjectFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&srcIP, "src-ip", "", "", "source node IP")
	cmd.Flags().StringVarP(&dstIP, "dst-ip", "", "", "destination node IP")
	cmd.Flags().StringVarP(&srcMAC, "src-mac", "", "", "source node MAC")
	cmd.Flags().StringVarP(&dstMAC, "dst-mac", "", "", "destination node MAC")
	cmd.Flags().Uint16VarP(&srcPort, "src-port", "", 0, "source port for TCP packet")
	cmd.Flags().Uint16VarP(&dstPort, "dst-port", "", 0, "destination port for TCP packet")
	cmd.Flags().StringVarP(&packetType, "type", "", "icmp4", "packet type: icmp4, icmp6, tcp4, tcp6, udp4 and udp6")
	cmd.Flags().StringVarP(&payload, "payload", "", "", "payload")
	cmd.Flags().StringVar(&pcap, "pcap", "", "PCAP file")
	cmd.Flags().Uint16VarP(&id, "id", "", 0, "ICMP identification")
	cmd.Flags().StringVarP(&mode, "mode", "", "unique", "specify mode of packet generation, `unique` or `random`")
	cmd.Flags().Int64VarP(&incrementPayload, "inc-payload", "", 0, "increase payload size each packet")
	cmd.Flags().Uint64VarP(&count, "count", "", 1, "number of packets to be generated")
	cmd.Flags().Uint64VarP(&interval, "interval", "", 0, "wait interval milliseconds between sending each packet")
	cmd.Flags().Uint8VarP(&ttl, "ttl", "", 64, "IP time-to-live header")
}

// GetPacketInjectRequest returns a packet injection request parsed from the command line flags
func GetPacketInjectRequest() (*pi.PacketInjectionRequest, error) {
	var pcapContent []byte
	if pcap != "" {
		f, err := os.Open(pcap)
		if err != nil {
			return nil, fmt.Errorf("Failed to open pcap file, err: %s", err)
		}

		pcapContent, err = ioutil.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("Failed to read pcap file, err: %s", err)
		}
	}

	var err error
	var sMAC, dMAC net.HardwareAddr
	if srcMAC != "" {
		if sMAC, err = net.ParseMAC(srcMAC); err != nil {
			return nil, fmt.Errorf("Source MAC parse error, err: %s", err)
		}
	}

	if dstMAC != "" {
		if dMAC, err = net.ParseMAC(dstMAC); err != nil {
			return nil, fmt.Errorf("Destination MAC parse error, err: %s", err)
		}
	}

	request := &packetinjector.PacketInjectionRequest{
		SrcIP:            net.ParseIP(srcIP),
		SrcMAC:           sMAC,
		SrcPort:          srcPort,
		DstIP:            net.ParseIP(dstIP),
		DstMAC:           dMAC,
		DstPort:          dstPort,
		Type:             packetType,
		Count:            count,
		ICMPID:           id,
		Interval:         interval,
		Mode:             mode,
		IncrementPayload: incrementPayload,
		Payload:          payload,
		Pcap:             pcapContent,
		TTL:              ttl,
	}

	if err := validator.Validate("packetinjection", request); err != nil {
		return nil, err
	}

	return request, nil
}

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

		request, err := GetPacketInjectRequest()
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
			Mode:             request.Mode,
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

		if err = validator.Validate("packetinjection", packet); err != nil {
			exitOnError(err)
		}

		ttl := 5 * time.Second
		if packet.Interval != 0 {
			ttl = time.Duration(packet.Interval*packet.Count)*time.Millisecond + 5*time.Second
		}
		createOpts := &http.CreateOptions{TTL: ttl}

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
	AddInjectPacketInjectFlags(PacketInjectionCreate)
}
