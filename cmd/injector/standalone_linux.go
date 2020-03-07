// +build linux

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
	"net"
	"os"
	"time"

	"github.com/skydive-project/go-debouncer"

	"github.com/skydive-project/skydive/cmd/client"
	pi "github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/rawsocket"
	"github.com/spf13/cobra"
)

var (
	encapType string
	ifName    string
)

// InjectPacketCmd injects packets without using any Skydive analyzer/agent
var InjectPacketCmd = &cobra.Command{
	Use:          "inject-packet",
	Short:        "inject packet",
	Long:         "inject packet",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		pir, err := client.GetPacketInjectRequest()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			cmd.Usage()
			os.Exit(1)
		}

		if ifName == "" {
			if ifName = getNameFromMac(pir.SrcMAC); ifName == "" {
				fmt.Fprintf(os.Stderr, "Could not find interface with MAC %s", pir.SrcMAC)
				os.Exit(1)
			}
		}

		rawSocket, err := rawsocket.NewRawSocket(ifName, rawsocket.OnlyIPPackets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open socket for interface %s, err: %s\n", ifName, err)
			os.Exit(1)
		}
		defer rawSocket.Close()

		var packetForger pi.PacketForger
		if len(pir.Pcap) > 0 {
			packetForger, err = pi.NewPcapPacketGenerator(pir)
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

		debouncer := debouncer.New(time.Second, showPacketCount)
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
	InjectPacketCmd.Flags().StringVarP(&ifName, "if-name", "", "", "interface to inject packets into")
	InjectPacketCmd.Flags().StringVarP(&encapType, "encap-type", "", "ether", "encapsulation type")
	client.AddInjectPacketInjectFlags(InjectPacketCmd)
}

func getNameFromMac(mac net.HardwareAddr) string {
	interfaces, _ := net.Interfaces()
	for _, ifi := range interfaces {
		if ifi.HardwareAddr.String() == mac.String() {
			return ifi.Name
		}
	}
	return ""
}
