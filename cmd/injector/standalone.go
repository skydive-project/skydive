/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package injector

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/packetinjector"
	pi "github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/validator"
	"github.com/spf13/cobra"
)

var (
	encapType        string
	ifName           string
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
	increment        bool
	incrementPayload int64
	ttl              uint8
)

// AddInjectPacketInjectFlags add the command line flags for a packet injection
func AddInjectPacketInjectFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&srcIP, "srcIP", "", "", "source node IP")
	cmd.Flags().StringVarP(&dstIP, "dstIP", "", "", "destination node IP")
	cmd.Flags().StringVarP(&srcMAC, "srcMAC", "", "", "source node MAC")
	cmd.Flags().StringVarP(&dstMAC, "dstMAC", "", "", "destination node MAC")
	cmd.Flags().Uint16VarP(&srcPort, "srcPort", "", 0, "source port for TCP packet")
	cmd.Flags().Uint16VarP(&dstPort, "dstPort", "", 0, "destination port for TCP packet")
	cmd.Flags().StringVarP(&packetType, "type", "", "icmp4", "packet type: icmp4, icmp6, tcp4, tcp6, udp4 and udp6")
	cmd.Flags().StringVarP(&payload, "payload", "", "", "payload")
	cmd.Flags().StringVar(&pcap, "pcap", "", "PCAP file")
	cmd.Flags().Uint16VarP(&id, "id", "", 0, "ICMP identification")
	cmd.Flags().BoolVarP(&increment, "increment", "", false, "increment ICMP id for each packet")
	cmd.Flags().Int64VarP(&incrementPayload, "incrementPayload", "", 0, "increase payload for each packet")
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
		Increment:        increment,
		IncrementPayload: incrementPayload,
		Payload:          payload,
		Pcap:             pcapContent,
		TTL:              ttl,
	}

	if err := validator.Validate(request); err != nil {
		return nil, err
	}

	return request, nil
}

// InjectPacketCmd injects packets without using any Skydive analyzer/agent
var InjectPacketCmd = &cobra.Command{
	Use:          "inject-packet",
	Short:        "inject packet",
	Long:         "inject packet",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		pir, err := GetPacketInjectRequest()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			cmd.Usage()
			os.Exit(1)
		}

		if ifName == "" {
			if ifName = getNameFromMac(srcMAC); ifName == "" {
				fmt.Fprintf(os.Stderr, "Could not find interface with MAC %s", srcMAC)
				os.Exit(1)
			}
		}

		rawSocket, err := common.NewRawSocket(ifName, common.OnlyIPPackets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open socket for interface %s, err: %s\n", ifName, err)
			os.Exit(1)
		}
		defer rawSocket.Close()

		var packetForger pi.PacketForger
		if len(pir.Pcap) > 0 {
			packetForger, err = pi.NewPcapPacketGenerator(pir.Pcap)
		} else {
			packetForger, err = pi.NewForgedPacketGenerator(pir, encapType)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create packet generator for interface %s, err: %s\n", ifName, err)
			os.Exit(1)
		}
		defer packetForger.Close()

		lastPacketCount := 0
		showPacketCount := func() {
			if lastPacketCount != 0 {
				fmt.Fprintf(os.Stderr, "%d packet(s) injected on interface %s\n", lastPacketCount, ifName)
			}
			lastPacketCount = 0
		}

		debouncer := common.NewDebouncer(time.Second, showPacketCount)
		debouncer.Start()
		defer debouncer.Stop()

		packetCount := 0
		for packet := range packetForger.PacketSource() {
			if packet == nil {
				break
			}

			if _, err := rawSocket.Write(packet.Data()); err != nil {
				fmt.Fprintf(os.Stderr, "Write error on interface %s: %s\n", ifName, err)
				os.Exit(1)
			}

			packetCount++
			lastPacketCount++
			debouncer.Call()
		}

		showPacketCount()

		fmt.Fprintf(os.Stderr, "%d packet(s) injected successfully\n", packetCount)
	},
}

func init() {
	InjectPacketCmd.Flags().StringVarP(&ifName, "ifName", "", "", "interface to inject packets into")
	InjectPacketCmd.Flags().StringVarP(&encapType, "encapType", "", "ether", "encapsulation type")
	AddInjectPacketInjectFlags(InjectPacketCmd)
}

func getNameFromMac(mac string) string {
	interfaces, _ := net.Interfaces()
	for _, ifi := range interfaces {
		if ifi.HardwareAddr.String() == mac {
			return ifi.Name
		}
	}
	return ""
}
