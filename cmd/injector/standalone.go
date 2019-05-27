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

	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
	pi "github.com/skydive-project/skydive/packetinjector"
)

func getNameFromMac(mac string) string {
	interfaces, _ := net.Interfaces()
	for _, ifi := range interfaces {
		if ifi.HardwareAddr.String() == mac {
			return ifi.Name
		}
	}
	return ""
}

// InjectPacket inject packet without agent
func InjectPacket(pip *pi.PacketInjectionParams) {
	var rawSocket *common.RawSocket
	ifName := getNameFromMac(pip.SrcMAC.String())
	protocol := 3

	if ifName == "" {
		fmt.Fprintf(os.Stderr, "Injection failed: interface has no name")
		return
	}

	rawSocket, err := common.NewRawSocket(ifName, protocol)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Injection failed: failed to open socket, err: %s\n", err)
		return
	}

	newInjector := &pi.PacketInjector{
		RawSocket: rawSocket,
		IfName:    ifName,
	}

	fpg := &pi.ForgedPacketGenerator{
		PacketInjectionParams: pip,
		LayerType:             layers.LayerTypeEthernet,
	}
	chnl := &pi.Channels{Pipes: make(map[string](chan bool))}
	_, done := pi.DoInjections(pip, chnl, newInjector, fpg, "")
	for range done {
		fmt.Fprintln(os.Stderr, "New packet injected")
	}
	fmt.Fprintln(os.Stderr, "Packet(s) Injected Successfully")
}
