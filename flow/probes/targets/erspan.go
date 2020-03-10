// +build linux

/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
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

package targets

/*
#ifdef __linux__
#define _GNU_SOURCE
#define __USE_GNU
#include <sched.h>
#include <string.h>
#include <stdlib.h>
#include <sys/socket.h>
#include <linux/sched.h>
#include <linux/if_packet.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <net/if.h>

static int open_raw_socket(const uint16_t protocol)
{
  int fd;

  fd = socket(AF_INET, SOCK_RAW | SOCK_NONBLOCK | SOCK_CLOEXEC, protocol);
  if (fd < 0)
    return 0;

  return fd;
}
#endif
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
	"strings"
	"syscall"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// ERSpanTarget defines a ERSpanTarget target
type ERSpanTarget struct {
	SessionID uint16
	IfIndex   uint32
	addr      syscall.Sockaddr
	fd        int
	buffer    gopacket.SerializeBuffer
	gre       *layers.GRE                  // for cache purpose
	layers    []gopacket.SerializableLayer // for cache purpose
	frame     *frame                       // for cache purpose
}

// https://tools.ietf.org/html/draft-foschiano-erspan
type headerII struct {
	Version   uint8
	Vlan      uint16
	Cos       uint8
	En        uint8
	Truncated uint8
	SessionID uint16
	Index     uint32
}

type frame struct {
	data []byte
}

var options = gopacket.SerializeOptions{
	ComputeChecksums: true,
	FixLengths:       true,
}

func (h *headerII) LayerType() gopacket.LayerType {
	return gopacket.LayerType(999)
}

func (h *headerII) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(8)
	if err != nil {
		return err
	}

	buf[0] = byte(uint16(h.Version)<<4 | (h.Vlan>>8)&0x0f)
	buf[1] = byte(h.Vlan & 0x00ff)

	flags := ((h.Cos&0x7)<<3 | (h.En&0x3)<<1 | h.Truncated)
	binary.BigEndian.PutUint16(buf[2:4], uint16((h.SessionID&0x2ff)|uint16(flags)<<10))
	binary.BigEndian.PutUint32(buf[4:8], uint32(h.Index&0xfffff))

	return nil
}

func (f *frame) LayerType() gopacket.LayerType {
	return gopacket.LayerType(999)
}

// SerializeTo implements gopacket interface
func (f *frame) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.AppendBytes(len(f.data) + 4)
	if err != nil {
		return err
	}
	copy(buf, f.data)

	binary.BigEndian.PutUint32(buf[len(f.data):], crc32.ChecksumIEEE(f.data))

	return nil
}

// SendPacket implements the Target interface
func (ers *ERSpanTarget) SendPacket(packet gopacket.Packet, bpf *flow.BPF) {
	if ers.buffer == nil {
		ers.buffer = gopacket.NewSerializeBuffer()

		ers.gre = &layers.GRE{
			Flags:    0x1,
			Protocol: 0x88be, //ERSPAN Type II
			Seq:      0,
		}
		ers.frame = &frame{}

		ers.layers = []gopacket.SerializableLayer{
			ers.gre,
			&headerII{
				Version:   0x1,
				En:        0x3,
				Truncated: 1,
				SessionID: ers.SessionID,
				Index:     ers.IfIndex,
			},
			ers.frame,
		}
	}
	ers.frame.data = packet.Data()

	if err := gopacket.SerializeLayers(ers.buffer, options, ers.layers...); err != nil {
		logging.GetLogger().Errorf("Error while serializing erspan packet: %s", err)
		ers.buffer.Clear()
		return
	}
	data := ers.buffer.Bytes()

	err := syscall.Sendto(ers.fd, data, 0, ers.addr)
	if err != nil {
		logging.GetLogger().Errorf("Write error erspan packet: %s", err)
	}
	ers.buffer.Clear()

	ers.gre.Seq++
}

// SendStats implements the Target interface
func (ers *ERSpanTarget) SendStats(stats flow.Stats) {
}

// Start start the target
func (ers *ERSpanTarget) Start() {
}

// Stop stops the target
func (ers *ERSpanTarget) Stop() {
	if ers.fd != 0 {
		syscall.Close(ers.fd)
	}
}

// NewERSpanTarget returns a new ERSpan target
func NewERSpanTarget(g *graph.Graph, n *graph.Node, capture *types.Capture) (*ERSpanTarget, error) {
	fd := C.open_raw_socket(C.uint16_t(syscall.IPPROTO_GRE))
	if fd == 0 {
		return nil, errors.New("Failed to open raw socket")
	}

	addr := syscall.SockaddrInet4{}

	ip := net.ParseIP(strings.Split(capture.Target, ":")[0]).To4()
	if ip == nil {
		return nil, fmt.Errorf("Invalid target address: %s", capture.Target)
	}
	copy(addr.Addr[:], ip)

	ifIndex, _ := n.GetFieldInt64("IfIndex")

	ers := &ERSpanTarget{
		SessionID: 1,
		IfIndex:   uint32(ifIndex),
		addr:      &addr,
		fd:        int(fd),
	}

	return ers, nil
}
