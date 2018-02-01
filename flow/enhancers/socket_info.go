// +build linux

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
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pmylund/go-cache"
	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

// SocketInfoEnhancer describes a SocketInfo Enhancer with TCP caches
type SocketInfoEnhancer struct {
	tracer *tracer.Tracer
	local  *cache.Cache
	remote *cache.Cache
}

// Name returns the SocketInfo enhancer name
func (s *SocketInfoEnhancer) Name() string {
	return "SocketInfo"
}

func ipToHex(ip46 net.IP) (s string) {
	ip := ip46.To4()
	if ip == nil {
		ip = ip46
	}

	for i := range ip {
		s += fmt.Sprintf("%02X", ip[(len(ip)-1)-i])
	}
	return
}

func hashIPPort(ip net.IP, port uint16) string {
	return fmt.Sprintf("%s:%04X", ipToHex(ip), port)
}

func getProcessInfo(pid int) (*flow.SocketInfo, error) {
	processPath := fmt.Sprintf("/proc/%d", pid)
	exe, err := os.Readlink(processPath + "/exe")
	if err != nil {
		return nil, err
	}

	stat, err := ioutil.ReadFile(processPath + "/stat")
	if err != nil {
		return nil, err
	}

	var name string
	fmt.Sscanf(string(stat), "%d %s", &pid, &name)

	socketInfo := &flow.SocketInfo{
		Process: exe,
		Name:    name[1 : len(name)-1],
		Pid:     int64(pid),
	}

	return socketInfo, nil
}

func (s *SocketInfoEnhancer) addEntry(srcAddr, dstAddr string, socketInfo *flow.SocketInfo) {
	s.local.Set(srcAddr, socketInfo, cache.NoExpiration)
	s.remote.Set(srcAddr+"/"+dstAddr, socketInfo, cache.NoExpiration)
}

func (s *SocketInfoEnhancer) removeEntry(srcAddr, dstAddr string) {
	// Delay deletion a bit
	if socketInfo, found := s.local.Get(srcAddr); found {
		s.local.Set(srcAddr, socketInfo, cache.DefaultExpiration)
	}
	if socketInfo, found := s.remote.Get(srcAddr + "/" + dstAddr); found {
		s.remote.Set(srcAddr+"/"+dstAddr, socketInfo, cache.DefaultExpiration)
	}
}

// TCPEventV4 is called when a tcp v4 event occurs
func (s *SocketInfoEnhancer) TCPEventV4(tcpV4 tracer.TcpV4) {
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
func (s *SocketInfoEnhancer) LostV4(uint64) {
}

// TCPEventV6 is called when a tcp v6 event occurs
func (s *SocketInfoEnhancer) TCPEventV6(tcpV6 tracer.TcpV6) {
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
func (s *SocketInfoEnhancer) LostV6(uint64) {
}

func (s *SocketInfoEnhancer) getSocketInfoLocal(f *flow.Flow) bool {
	var ipPortA, ipPortB string
	var mappedA, mappedB bool
	var entry interface{}

	ip := net.ParseIP(f.Network.A)
	if ip == nil {
		return false
	}

	port, _ := strconv.ParseUint(f.Transport.A, 10, 0)
	ipPortA = hashIPPort(ip, uint16(port))

	ip = net.ParseIP(f.Network.B)
	if ip != nil {
		port, _ = strconv.ParseUint(f.Transport.B, 10, 0)
		ipPortB = hashIPPort(ip, uint16(port))
	}

	if entry, mappedA = s.local.Get(ipPortA); mappedA {
		f.SocketA = entry.(*flow.SocketInfo)
	}

	// peer (si.LocalIPPort == ipPortB) && (si.RemoteIPPort == ipPortA)
	if entry, mappedB = s.remote.Get(ipPortB + "/" + ipPortA); mappedB {
		f.SocketB = entry.(*flow.SocketInfo)
	}

	return mappedA || mappedB
}

func (s *SocketInfoEnhancer) scanProc() error {
	s.local.Flush()
	s.remote.Flush()

	inodePids := make(map[int]int)
	namespaces := make(map[uint64]bool)
	processes := make(map[int]*flow.SocketInfo)

	buildInodePidMap := func() error {
		// Loop through all fd dirs of process on /proc to compare the inode and
		// get the pid. Shamelessly taken from github.com/drael/GOnetstat
		d, err := filepath.Glob("/proc/[0-9]*/fd/[0-9]*")
		if err != nil {
			return err
		}

		var inode int
		for _, item := range d {
			path, _ := os.Readlink(item)
			if _, err := fmt.Sscanf(path, "socket:[%d]", &inode); err == nil {
				pid, _ := strconv.Atoi(strings.Split(item, "/")[2])
				inodePids[inode] = pid
			}
		}

		return nil
	}

	parseNetTCP := func(path string) {
		u, err := os.Open(path)
		if err != nil {
			return
		}
		defer u.Close()

		var stats syscall.Stat_t
		if err := syscall.Fstat(int(u.Fd()), &stats); err != nil {
			return
		}

		if _, found := namespaces[stats.Ino]; found {
			// Already parsed
			return
		}
		namespaces[stats.Ino] = true

		r := bufio.NewReader(u)
		r.ReadLine()

		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}

			var localIPPort, remoteIPPort string
			var inode, ignore int

			_, err = fmt.Sscanf(line, "%d: %s %s %x %x:%x %x:%x %x %d %d %d",
				&ignore, &localIPPort, &remoteIPPort,
				&ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore,
				&inode)

			if err != nil {
				logging.GetLogger().Debugf("Failed to parse %s: %s", path, err.Error())
				continue
			}

			pid, found := inodePids[inode]
			if !found {
				logging.GetLogger().Debugf("Could not find process for inode %d", inode)
				continue
			}

			socketInfo, found := processes[pid]
			if !found {
				socketInfo, err = getProcessInfo(pid)
				if err != nil {
					logging.GetLogger().Debugf("Failed to get stats for process %d", pid)
					continue
				}
				processes[pid] = socketInfo
			}

			s.addEntry(localIPPort, remoteIPPort, socketInfo)
		}
	}

	if err := buildInodePidMap(); err != nil {
		return err
	}

	d, err := filepath.Glob("/proc/[0-9]*/task/[0-9]*/net")
	if err != nil {
		return err
	}

	for _, item := range d {
		parseNetTCP(item + "/tcp")
		parseNetTCP(item + "/tcp6")
	}

	return nil
}

func (s *SocketInfoEnhancer) getFlowSocketInfoLocal(f *flow.Flow) {
	if found := s.getSocketInfoLocal(f); !found {
		if s.tracer == nil {
			s.scanProc()
			if found = s.getSocketInfoLocal(f); found {
				return
			}
		}

		f.SkipSocketInfo(true)
	}
}

// Enhance the flow with process info
func (s *SocketInfoEnhancer) Enhance(f *flow.Flow) {
	if f.Transport == nil || f.SkipSocketInfo() || f.Transport.Protocol != flow.FlowProtocol_TCPPORT {
		return
	}

	if f.SocketA == nil && f.SocketB == nil {
		s.getFlowSocketInfoLocal(f)
	}
}

// Start the flow enhancer
func (s *SocketInfoEnhancer) Start() error {
	if s.tracer != nil {
		s.tracer.Start()
	}

	return s.scanProc()
}

// Stop the flow enhancer
func (s *SocketInfoEnhancer) Stop() {
	if s.tracer != nil {
		s.tracer.Stop()
	}
}

// NewSocketInfoEnhancer create a new SocketInfo Enhancer
func NewSocketInfoEnhancer(expire, cleanup time.Duration) *SocketInfoEnhancer {
	s := &SocketInfoEnhancer{
		local:  cache.New(expire, cleanup),
		remote: cache.New(expire, cleanup),
	}

	var err error
	if s.tracer, err = tracer.NewTracer(s); err != nil {
		logging.GetLogger().Infof("Socket info enhancer is running in compatibility mode: %s", err.Error())
	}

	return s
}
