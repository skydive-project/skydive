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

package enhancers

import (
	"time"

	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

// EBPFSocketInfoEnhancer describes a eBPF based socket mapper
type EBPFSocketInfoEnhancer struct {
	*ProcSocketInfoEnhancer
	tracer *tracer.Tracer
}

// TCPEventV4 is called when a tcp v4 event occurs
func (s *EBPFSocketInfoEnhancer) TCPEventV4(tcpV4 tracer.TcpV4) {
	srcAddr := hashIPPort(tcpV4.SAddr, tcpV4.SPort)
	dstAddr := hashIPPort(tcpV4.DAddr, tcpV4.DPort)

	switch tcpV4.Type {
	case tracer.EventConnect, tracer.EventAccept:
		if socketInfo, err := getProcessInfo(int(tcpV4.Pid)); err == nil {
			s.addEntry(srcAddr, dstAddr, socketInfo)
		}
	case tracer.EventClose:
		s.removeEntry(srcAddr, dstAddr)
	}
}

// LostV4 is called when a tcp v4 event was lost
func (s *EBPFSocketInfoEnhancer) LostV4(uint64) {
}

// TCPEventV6 is called when a tcp v6 event occurs
func (s *EBPFSocketInfoEnhancer) TCPEventV6(tcpV6 tracer.TcpV6) {
	srcAddr := hashIPPort(tcpV6.SAddr, tcpV6.SPort)
	dstAddr := hashIPPort(tcpV6.DAddr, tcpV6.DPort)

	switch tcpV6.Type {
	case tracer.EventConnect, tracer.EventAccept:
		if socketInfo, err := getProcessInfo(int(tcpV6.Pid)); err == nil {
			s.addEntry(srcAddr, dstAddr, socketInfo)
		}
	case tracer.EventClose:
	}
}

// LostV6 is called when a tcp v6 event was lost
func (s *EBPFSocketInfoEnhancer) LostV6(uint64) {
}

// Enhance the flow with process info
func (s *EBPFSocketInfoEnhancer) Enhance(f *flow.Flow) {
	if f.Transport == nil || f.SkipSocketInfo() || f.Transport.Protocol != flow.FlowProtocol_TCPPORT {
		return
	}

	if f.SocketA == nil && f.SocketB == nil {
		if !s.mapFlow(f) {
			f.SkipSocketInfo(true)
		}
	}
}

// Start the flow enhancer
func (s *EBPFSocketInfoEnhancer) Start() error {
	s.tracer.Start()
	return s.scanProc()
}

// Stop the flow enhancer
func (s *EBPFSocketInfoEnhancer) Stop() {
	s.tracer.Stop()
}

// NewSocketInfoEnhancer create a new SocketInfo Enhancer
func NewSocketInfoEnhancer(expire, cleanup time.Duration) flow.Enhancer {
	s := &EBPFSocketInfoEnhancer{
		ProcSocketInfoEnhancer: NewProcSocketInfoEnhancer(expire, cleanup),
	}

	var err error
	if s.tracer, err = tracer.NewTracer(s); err != nil {
		logging.GetLogger().Infof("Socket info enhancer is running in compatibility mode: %s", err.Error())
		return s.ProcSocketInfoEnhancer
	}

	return s
}
