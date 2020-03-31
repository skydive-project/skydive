// +build linux,ebpf

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package socketinfo

import (
	"net"
	"time"

	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/probe"
	tp "github.com/skydive-project/skydive/topology/probes"
)

// EBPFProbe describes a eBPF based socket mapper
type EBPFProbe struct {
	*ProcProbe
	tracer *tracer.Tracer
}

// TCPEventV4 is called when a TCPv4 event occurs
func (s *EBPFProbe) TCPEventV4(tcpV4 tracer.TcpV4) {
	srcAddr := &net.TCPAddr{IP: tcpV4.SAddr.To4(), Port: int(tcpV4.SPort)}
	dstAddr := &net.TCPAddr{IP: tcpV4.DAddr.To4(), Port: int(tcpV4.DPort)}

	if tcpV4.Type == tracer.EventClose {
		s.connCache.Remove(flow.FlowProtocol_TCP, srcAddr, dstAddr)
		return
	}

	if processInfo, err := getProcessInfo(int(tcpV4.Pid)); err == nil {
		conn := &ConnectionInfo{
			ProcessInfo:   *processInfo,
			LocalAddress:  srcAddr.IP.String(),
			LocalPort:     int64(srcAddr.Port),
			RemoteAddress: dstAddr.IP.String(),
			RemotePort:    int64(dstAddr.Port),
			State:         ConnectionState(StateEstablished),
			Protocol:      flow.FlowProtocol_TCP,
		}
		s.connCache.Set(conn.Hash(), conn)

		if tcpV4.Type == tracer.EventAccept {
			listenConn := *conn
			listenConn.State = StateListen
			listenConn.RemoteAddress = "0.0.0.0"
			listenConn.RemotePort = 0
			s.connCache.Set(listenConn.Hash(), &listenConn)
		}
	}
}

// LostV4 is called when a TCPv4 event was lost
func (s *EBPFProbe) LostV4(uint64) {
}

// TCPEventV6 is called when a TCPv6 event occurs
func (s *EBPFProbe) TCPEventV6(tcpV6 tracer.TcpV6) {
	srcAddr := &net.TCPAddr{IP: tcpV6.SAddr.To16(), Port: int(tcpV6.SPort)}
	dstAddr := &net.TCPAddr{IP: tcpV6.DAddr.To16(), Port: int(tcpV6.DPort)}

	if tcpV6.Type == tracer.EventClose {
		s.connCache.Remove(flow.FlowProtocol_TCP, srcAddr, dstAddr)
		return
	}

	if processInfo, err := getProcessInfo(int(tcpV6.Pid)); err == nil {
		conn := &ConnectionInfo{
			ProcessInfo:   *processInfo,
			LocalAddress:  srcAddr.IP.String(),
			LocalPort:     int64(srcAddr.Port),
			RemoteAddress: dstAddr.IP.String(),
			RemotePort:    int64(dstAddr.Port),
			State:         ConnectionState(StateEstablished),
			Protocol:      flow.FlowProtocol_TCP,
		}
		s.connCache.Set(conn.Hash(), conn)

		if tcpV6.Type == tracer.EventAccept {
			listenConn := *conn
			listenConn.State = StateListen
			listenConn.RemoteAddress = "::/0"
			listenConn.RemotePort = 0
			s.connCache.Set(listenConn.Hash(), &listenConn)
		}
	}
}

// LostV6 is called when a TCPv6 event was lost
func (s *EBPFProbe) LostV6(uint64) {
}

// Start the flow Probe
func (s *EBPFProbe) Start() error {
	s.tracer.Start()

	s.scanProc()
	s.updateMetadata()

	go func() {
		seconds := s.Ctx.Config.GetInt("agent.topology.socketinfo.host_update")
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

	return nil
}

// Stop the flow Probe
func (s *EBPFProbe) Stop() {
	s.ProcProbe.Stop()
	s.tracer.Stop()
}

// NewProbe returns a new socket info topology probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	// /proc is used for initialization
	procProbe := NewProcProbe(ctx)

	p := &EBPFProbe{
		ProcProbe: procProbe,
	}

	probeHandler := &ProbeHandler{
		Handler: p,
	}

	var err error
	if p.tracer, err = tracer.NewTracer(p); err != nil {
		ctx.Logger.Infof("Socket info probe is running in compatibility mode: %s", err)
		probeHandler.Handler = procProbe
	}

	return probeHandler, nil
}
