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

package socketinfo

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/topology/graph"
)

var tcpStates = []string{
	"UNKNOWN",
	"ESTABLISHED",
	"SYN_SENT",
	"SYN_RECV",
	"FIN_WAIT1",
	"FIN_WAIT2",
	"TIME_WAIT",
	"CLOSE",
	"CLOSE_WAIT",
	"LAST_ACK",
	"LISTEN",
	"CLOSING",
}

// ProcSocketInfoProbe describes a probe that collects active connections
type ProcSocketInfoProbe struct {
	connCache *ConnectionCache
	quit      chan bool
	graph     *graph.Graph
	host      *graph.Node
	procGlob  string
}

func getProcessInfo(pid int) (*ProcessInfo, error) {
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

	return &ProcessInfo{
		Process: exe,
		Name:    name[1 : len(name)-1],
		Pid:     int64(pid),
	}, nil
}

func (s *ProcSocketInfoProbe) scanProc() error {
	s.connCache.Flush()

	inodePids := make(map[int]int)
	namespaces := make(map[uint64]bool)
	processes := make(map[int]*ProcessInfo)

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

	parseProcAddr := func(hexIPPort string) (ip net.IP, port uint16) {
		splitted := strings.SplitN(hexIPPort, ":", 2)
		hexIP, hexPort := splitted[0], splitted[1]
		ipString, _ := hex.DecodeString(hexIP)
		portString, _ := hex.DecodeString(hexPort)
		port = binary.BigEndian.Uint16(portString)

		if len(hexIP) > 8 {
			ip = make(net.IP, 16)
			n := binary.LittleEndian.Uint32(ipString[0:4])
			binary.BigEndian.PutUint32(ip[0:], n)
			n = binary.LittleEndian.Uint32(ipString[4:8])
			binary.BigEndian.PutUint32(ip[4:], n)
			n = binary.LittleEndian.Uint32(ipString[8:12])
			binary.BigEndian.PutUint32(ip[8:], n)
			n = binary.LittleEndian.Uint32(ipString[12:16])
			binary.BigEndian.PutUint32(ip[12:], n)
		} else {
			ip = make(net.IP, 4)
			n := binary.LittleEndian.Uint32(ipString)
			binary.BigEndian.PutUint32(ip, n)
		}
		return
	}

	parseNetEntry := func(line string) (*ConnectionInfo, error) {
		var localIPPort, remoteIPPort string
		var inode, ignore, state int

		_, err := fmt.Sscanf(line, "%d: %s %s %x %x:%x %x:%x %x %d %d %d",
			&ignore, &localIPPort, &remoteIPPort,
			&state, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore,
			&inode)
		if err != nil {
			return nil, err
		}

		localIP, localPort := parseProcAddr(localIPPort)
		remoteIP, remotePort := parseProcAddr(remoteIPPort)

		pid, found := inodePids[inode]
		if !found {
			return nil, fmt.Errorf("Could not find process for inode %d", inode)
		}

		processInfo, found := processes[pid]
		if !found {
			processInfo, err = getProcessInfo(pid)
			if err != nil {
				return nil, fmt.Errorf("Failed to get stats for process %d", pid)
			}
			processes[pid] = processInfo
		}

		return &ConnectionInfo{
			ProcessInfo:   *processInfo,
			LocalAddress:  localIP.String(),
			LocalPort:     int64(localPort),
			RemoteAddress: remoteIP.String(),
			RemotePort:    int64(remotePort),
			State:         ConnectionState(tcpStates[state]),
		}, nil
	}

	parseNet := func(protocol flow.FlowProtocol, path string) {
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

			conn, err := parseNetEntry(line)
			if err != nil {
				continue
			}
			conn.Protocol = protocol

			s.connCache.Set(conn.Hash(), conn)
		}
	}

	parseNetTCP := func(path string) {
		parseNet(flow.FlowProtocol_TCP, path)
	}

	parseNetUDP := func(path string) {
		parseNet(flow.FlowProtocol_UDP, path)
	}

	if err := buildInodePidMap(); err != nil {
		return err
	}

	d, err := filepath.Glob(s.procGlob)
	if err != nil {
		return err
	}

	for _, item := range d {
		parseNetTCP(item + "/tcp")
		parseNetTCP(item + "/tcp6")
		parseNetUDP(item + "/udp")
		parseNetUDP(item + "/udp6")
	}

	return nil
}

func (s *ProcSocketInfoProbe) updateMetadata() {
	var sockets []*ConnectionInfo
	for _, item := range s.connCache.Items() {
		conn := item.Object.(*ConnectionInfo)
		sockets = append(sockets, conn)
	}

	s.graph.Lock()
	s.graph.AddMetadata(s.host, "Sockets", sockets)
	s.graph.Unlock()
}

// MapTCP returns the sending and receiving processes for a pair of TCP addresses
// When using /proc, if the connection was not found at the first try, we scan
// /proc again
func (s *ProcSocketInfoProbe) MapTCP(srcAddr, dstAddr *net.TCPAddr) (src *ProcessInfo, dst *ProcessInfo) {
	if src, dst = s.connCache.MapTCP(srcAddr, dstAddr); src == nil && dst == nil {
		s.scanProc()
		src, dst = s.connCache.MapTCP(srcAddr, dstAddr)
	}
	return
}

// Start the socket info probe
func (s *ProcSocketInfoProbe) Start() {
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
				s.scanProc()
				s.updateMetadata()
			}
		}
	}()
}

// Stop the socket info probe
func (s *ProcSocketInfoProbe) Stop() {
	s.quit <- true
}

// NewProcSocketInfoProbe create a new socket info probe
func NewProcSocketInfoProbe(g *graph.Graph, host *graph.Node) *ProcSocketInfoProbe {
	procGlob := "/proc/[0-9]*/task/[0-9]*/net"
	pid := os.Getpid()
	if _, err := os.Stat(fmt.Sprintf("/proc/%d/tasks/%d/net", pid, pid)); os.IsNotExist(err) {
		procGlob = "/proc/[0-9]*/net"
	}

	return &ProcSocketInfoProbe{
		procGlob:  procGlob,
		connCache: NewConnectionCache(),
		quit:      make(chan bool),
		graph:     g,
		host:      host,
	}
}
