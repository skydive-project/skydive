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

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

// SocketInfoEnhancer describes a based socket mapper
type ProcSocketInfoEnhancer struct {
	local  *cache.Cache
	remote *cache.Cache
}

// Name returns the SocketInfo enhancer name
func (s *ProcSocketInfoEnhancer) Name() string {
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

func (s *ProcSocketInfoEnhancer) addEntry(srcAddr, dstAddr string, socketInfo *flow.SocketInfo) {
	s.local.Set(srcAddr, socketInfo, cache.NoExpiration)
	s.remote.Set(srcAddr+"/"+dstAddr, socketInfo, cache.NoExpiration)
}

func (s *ProcSocketInfoEnhancer) removeEntry(srcAddr, dstAddr string) {
	// Delay deletion a bit
	if socketInfo, found := s.local.Get(srcAddr); found {
		s.local.Set(srcAddr, socketInfo, cache.DefaultExpiration)
	}
	if socketInfo, found := s.remote.Get(srcAddr + "/" + dstAddr); found {
		s.remote.Set(srcAddr+"/"+dstAddr, socketInfo, cache.DefaultExpiration)
	}
}

func (s *ProcSocketInfoEnhancer) mapFlow(f *flow.Flow) bool {
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

func (s *ProcSocketInfoEnhancer) scanProc() error {
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

// Enhance the flow with process info
func (s *ProcSocketInfoEnhancer) Enhance(f *flow.Flow) {
	if f.Transport == nil || f.SkipSocketInfo() || f.Transport.Protocol != flow.FlowProtocol_TCPPORT {
		return
	}

	if f.SocketA == nil && f.SocketB == nil {
		if found := s.mapFlow(f); !found {
			s.scanProc()
			if found = s.mapFlow(f); found {
				return
			}
		}
		f.SkipSocketInfo(true)
	}
}

// Start the flow enhancer
func (s *ProcSocketInfoEnhancer) Start() error {
	return s.scanProc()
}

// Stop the flow enhancer
func (s *ProcSocketInfoEnhancer) Stop() {
}

// NewSocketInfoEnhancer create a new SocketInfo Enhancer
func NewProcSocketInfoEnhancer(expire, cleanup time.Duration) *ProcSocketInfoEnhancer {
	return &ProcSocketInfoEnhancer{
		local:  cache.New(expire, cleanup),
		remote: cache.New(expire, cleanup),
	}
}
