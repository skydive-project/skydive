// +build linux,ebpf

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package socketinfo

import (
	"net"
	"time"

	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

// EBPFSocketInfoProbe describes a eBPF based socket mapper
type EBPFSocketInfoProbe struct {
	*ProcSocketInfoProbe
	tracer *tracer.Tracer
}

// TCPEventV4 is called when a TCPv4 event occurs
func (s *EBPFSocketInfoProbe) TCPEventV4(tcpV4 tracer.TcpV4) {
	srcAddr := &net.TCPAddr{IP: tcpV4.SAddr.To4(), Port: int(tcpV4.SPort)}
	dstAddr := &net.TCPAddr{IP: tcpV4.DAddr.To4(), Port: int(tcpV4.DPort)}

	logging.GetLogger().Debugf("Got new TCPv4 event: %+v", tcpV4)

	switch tcpV4.Type {
	case tracer.EventConnect, tracer.EventAccept:
		if processInfo, err := getProcessInfo(int(tcpV4.Pid)); err == nil {
			conn := &ConnectionInfo{
				ProcessInfo:   *processInfo,
				LocalAddress:  srcAddr.IP.String(),
				LocalPort:     int64(srcAddr.Port),
				RemoteAddress: dstAddr.IP.String(),
				RemotePort:    int64(dstAddr.Port),
				State:         ConnectionState(tcpStates[1]),
				Protocol:      flow.FlowProtocol_TCP,
			}
			s.connCache.Set(conn.Hash(), conn)
		}
	case tracer.EventClose:
		s.connCache.Remove(flow.FlowProtocol_TCP, srcAddr, dstAddr)
	}
}

// LostV4 is called when a TCPv4 event was lost
func (s *EBPFSocketInfoProbe) LostV4(uint64) {
}

// TCPEventV6 is called when a TCPv6 event occurs
func (s *EBPFSocketInfoProbe) TCPEventV6(tcpV6 tracer.TcpV6) {
	srcAddr := &net.TCPAddr{IP: tcpV6.SAddr.To16(), Port: int(tcpV6.SPort)}
	dstAddr := &net.TCPAddr{IP: tcpV6.DAddr.To16(), Port: int(tcpV6.DPort)}

	switch tcpV6.Type {
	case tracer.EventConnect, tracer.EventAccept:
		if processInfo, err := getProcessInfo(int(tcpV6.Pid)); err == nil {
			conn := &ConnectionInfo{
				ProcessInfo:   *processInfo,
				LocalAddress:  srcAddr.IP.String(),
				LocalPort:     int64(srcAddr.Port),
				RemoteAddress: dstAddr.IP.String(),
				RemotePort:    int64(dstAddr.Port),
				State:         ConnectionState(tcpStates[1]),
				Protocol:      flow.FlowProtocol_TCP,
			}
			s.connCache.Set(conn.Hash(), conn)
		}
	case tracer.EventClose:
		s.connCache.Remove(flow.FlowProtocol_TCP, srcAddr, dstAddr)
	}
}

// LostV6 is called when a TCPv6 event was lost
func (s *EBPFSocketInfoProbe) LostV6(uint64) {
}

// Start the flow Probe
func (s *EBPFSocketInfoProbe) Start() {
	s.tracer.Start()

	s.scanProc()
	s.updateMetadata()

	go func() {
		seconds := config.GetInt("agent.topology.socketinfo.host_update")
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.quit:
				return
			case <-ticker.C:
				s.updateMetadata()
			}
		}
	}()
}

// Stop the flow Probe
func (s *EBPFSocketInfoProbe) Stop() {
	s.ProcSocketInfoProbe.Stop()
	s.tracer.Stop()
}

// NewSocketInfoProbe create a new SocketInfo Probe
func NewSocketInfoProbe(g *graph.Graph, host *graph.Node) probe.Probe {
	s := &EBPFSocketInfoProbe{
		ProcSocketInfoProbe: NewProcSocketInfoProbe(g, host),
	}

	var err error
	if s.tracer, err = tracer.NewTracer(s); err != nil {
		logging.GetLogger().Infof("Socket info probe is running in compatibility mode: %s", err.Error())
		return s.ProcSocketInfoProbe
	}

	return s
}
