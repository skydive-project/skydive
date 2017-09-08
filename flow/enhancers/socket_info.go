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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/flow"
)

// SocketInfo describes a process and socket information
type SocketInfo struct {
	Uid          int
	LocalIPPort  string
	RemoteIPPort string

	Pid  int
	PPid int
	Tid  int
	Exe  string
	Name string
}

// SocketInfoEnhancer describes a SocketInfo Enhancer with UDP/TCP caches
type SocketInfoEnhancer struct {
	cacheTCP      *socketCache
	cacheUDP      *socketCache
	inodeNetMutex sync.Mutex
	inodeNet      map[string]bool
}

type socketCache struct {
	inode           *cache.Cache
	localAddr       *cache.Cache
	localRemoteAddr *cache.Cache
}

type cacheAddr struct {
	inode string
}

// Name return the SocketInfo enahancer name
func (s *SocketInfoEnhancer) Name() string {
	return "SocketInfo"
}

func (s *SocketInfoEnhancer) resetScanProcNet() {
	s.inodeNetMutex.Lock()
	s.inodeNet = make(map[string]bool)
	s.inodeNetMutex.Unlock()
}

func (s *SocketInfoEnhancer) needScanProcNet(procProcessRoot string) bool {
	nsNet, err := os.Readlink(procProcessRoot + "/ns/net")
	if err != nil {
		return false
	}

	s.inodeNetMutex.Lock()
	defer s.inodeNetMutex.Unlock()
	if _, ok := s.inodeNet[nsNet]; !ok {
		s.inodeNet[nsNet] = true
		return true
	}
	return false
}

func (s *SocketInfoEnhancer) parseNetUDPTCP(c *socketCache, path string) {
	u, err := os.Open(path)
	if err != nil {
		return
	}
	defer u.Close()

	r := bufio.NewReader(u)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		var localIPPort, remoteIPPort string
		var state, wq, rq int
		var timer, timeout, retrs, uid, probes int
		var ino uint64
		var refcnt, sk, rto, ato, qack int
		var cwnd, ssthresh, opt int

		fmt.Sscanf(line[i+1:], "%s %s %x %x:%x %x:%x %x %d %d %d %d %llx %d %d %d %d %d %[^\n]\n",
			&localIPPort, &remoteIPPort,
			&state, &wq, &rq,
			&timer, &timeout, &retrs, &uid, &probes, &ino,
			&refcnt, &sk, &rto, &ato, &qack,
			&cwnd, &ssthresh, &opt)

		inode := strconv.FormatUint(ino, 10)
		_, ok := c.inode.Get(inode)
		if ok {
			continue
		}
		si := SocketInfo{
			Uid:          uid,
			LocalIPPort:  localIPPort,
			RemoteIPPort: remoteIPPort,
		}
		c.inode.Set(inode, si, cache.DefaultExpiration)
		c.localAddr.Set(localIPPort, cacheAddr{inode: inode}, cache.DefaultExpiration)
		c.localRemoteAddr.Set(localIPPort+remoteIPPort, cacheAddr{inode: inode}, cache.DefaultExpiration)
	}
}

func (s *SocketInfoEnhancer) updateProcNet(procProcessRoot string) {
	/* scan only new one based on network namespace */
	if s.needScanProcNet(procProcessRoot) == false {
		return
	}
	s.parseNetUDPTCP(s.cacheUDP, procProcessRoot+"/net/udp")
	s.parseNetUDPTCP(s.cacheUDP, procProcessRoot+"/net/udp6")
	s.parseNetUDPTCP(s.cacheTCP, procProcessRoot+"/net/tcp")
	s.parseNetUDPTCP(s.cacheTCP, procProcessRoot+"/net/tcp6")
}

var rProcPidTid = regexp.MustCompile("/proc/(\\d+)/task/(\\d+)$")

func parseProcStat(procProcessRoot string, si *SocketInfo) {
	// Retreive binary name
	exe, err := os.Readlink(procProcessRoot + "/exe")
	if err != nil {
		return
	}
	si.Exe = exe

	// Retreive task info
	if strings.Contains(procProcessRoot, "/task/") == true {
		p := rProcPidTid.FindStringSubmatch(procProcessRoot)
		if len(p) > 0 {
			pid, _ := strconv.Atoi(p[1])
			tid, _ := strconv.Atoi(p[2])
			si.Pid = pid
			si.Tid = tid
		}
	}

	// Retreive ppid and task name
	stat, err := ioutil.ReadFile(procProcessRoot + "/stat")
	if err != nil {
		return
	}
	var pid, ppid, status int
	var name string
	fmt.Sscanf(string(stat), "%d %s %c %d", &pid, &name, &status, &ppid)
	if si.Pid == 0 {
		si.Pid = pid
	}
	si.Name = name[1 : len(name)-1]
	si.PPid = ppid
}

func getPIDs(path string) []string {
	pids := []string{}
	proc, err := os.Open(path)
	if err != nil {
		return pids
	}
	defer proc.Close()
	names, err := proc.Readdirnames(0)
	if err != nil {
		return pids
	}
	for _, n := range names {
		if _, err := strconv.ParseInt(n, 0, 0); err != nil {
			continue
		}
		pids = append(pids, n)
	}
	return pids
}

func getFDs(path string) []string {
	fds := []string{}
	proc, err := os.Open(path + "/fd")
	if err != nil {
		return fds
	}
	defer proc.Close()
	names, err := proc.Readdirnames(0)
	if err != nil {
		return fds
	}
	for _, n := range names {
		fd, err := os.Readlink(path + "/fd/" + n)
		if err != nil {
			continue
		}
		fds = append(fds, fd)
	}
	return fds
}

var rFdSocket = regexp.MustCompile("socket:\\[(\\d+)\\]$")

func (s *SocketInfoEnhancer) scanProc() {
	s.resetScanProcNet()

	pids := getPIDs("/proc")
	for _, p := range pids {
		procTask := "/proc/" + p + "/task"
		tasks := getPIDs(procTask)
		for _, t := range tasks {
			process := procTask + "/" + t
			s.updateProcNet(process)

			fds := getFDs(process)
			for _, f := range fds {
				inode := rFdSocket.FindStringSubmatch(f)
				if len(inode) == 0 {
					continue
				}

				var isUDP = true
				sinfo, ok := s.cacheUDP.inode.Get(inode[1])
				if !ok {
					isUDP = false
					sinfo, ok = s.cacheTCP.inode.Get(inode[1])
					if !ok {
						continue
					}
				}
				si := sinfo.(SocketInfo)
				parseProcStat(process, &si)
				if isUDP == true {
					s.cacheUDP.inode.Set(inode[1], si, cache.DefaultExpiration)
				} else {
					s.cacheTCP.inode.Set(inode[1], si, cache.DefaultExpiration)
				}
			}
		}
	}
}

func ipToHex(ip46 net.IP) string {
	s := ""
	ip := ip46.To4()
	if ip == nil {
		ip = ip46
	}
	for i := range ip {
		s += fmt.Sprintf("%02X", ip[(len(ip)-1)-i])
	}
	return s
}

func (s *SocketInfoEnhancer) getSocketInfoLocal(f *flow.Flow) bool {
	ip := net.ParseIP(f.Network.A)
	if ip == nil {
		return false
	}

	port, _ := strconv.ParseUint(f.Transport.A, 10, 0)
	ipPortA := fmt.Sprintf("%s:%04X", ipToHex(ip), port)
	ipPortB := ""

	ip = net.ParseIP(f.Network.B)
	if ip != nil {
		port, _ = strconv.ParseUint(f.Transport.B, 10, 0)
		ipPortB = fmt.Sprintf("%s:%04X", ipToHex(ip), port)
	}

	var cache *socketCache
	if f.Transport.Protocol == flow.FlowProtocol_UDPPORT {
		cache = s.cacheUDP
	} else if f.Transport.Protocol == flow.FlowProtocol_TCPPORT {
		cache = s.cacheTCP
	}

	retried := false
retry:
	var siA, siB bool
	// si.LocalIPPort == ipPortA
	if local, ok := cache.localAddr.Get(ipPortA); ok {
		if ca, ok := local.(cacheAddr); ok {
			if sinfo, ok := cache.inode.Get(ca.inode); ok {
				if si, ok := sinfo.(SocketInfo); ok {
					f.SocketA = SocketInfoToFlowSocketInfo(&si)
					siA = true
				}
			}
		}
	}
	// peer (si.LocalIPPort == ipPortB) && (si.RemoteIPPort == ipPortA)
	if local, ok := cache.localRemoteAddr.Get(ipPortB + ipPortA); ok {
		if ca, ok := local.(cacheAddr); ok {
			if sinfo, ok := cache.inode.Get(ca.inode); ok {
				if si, ok := sinfo.(SocketInfo); ok {
					f.SocketB = SocketInfoToFlowSocketInfo(&si)
					siB = true
				}
			}
		}
	}
	if siA || siB {
		return true
	}

	if retried == false {
		s.scanProc()
		retried = true
		goto retry
	}
	return false
}

// SocketInfoToFlowSocketInfo create a new flow.SocketInfo based on process SocketInfo
func SocketInfoToFlowSocketInfo(si *SocketInfo) *flow.SocketInfo {
	if si == nil {
		return nil
	}
	return &flow.SocketInfo{
		Process: si.Exe,
		Pid:     int64(si.Pid),
		Ppid:    int64(si.PPid),
		Name:    si.Name,
		Tid:     int64(si.Tid),
	}
}

func (s *SocketInfoEnhancer) getFlowSocketInfoLocal(f *flow.Flow) {
	if foundSI := s.getSocketInfoLocal(f); foundSI == false {
		f.SkipSocketInfo(true)
		return
	}
	return
}

// Enhance the graph with process info
func (s *SocketInfoEnhancer) Enhance(f *flow.Flow) {
	if f.Transport == nil || f.SkipSocketInfo() || f.Transport.Protocol == flow.FlowProtocol_SCTPPORT {
		return
	}
	if f.SocketA == nil && f.SocketB == nil {
		s.getFlowSocketInfoLocal(f)
	}
}

func newSocketCache(expire, cleanup time.Duration) *socketCache {
	return &socketCache{
		inode:           cache.New(expire, cleanup),
		localAddr:       cache.New(expire, cleanup),
		localRemoteAddr: cache.New(expire, cleanup),
	}
}

// NewSocketInfoEnhancer create a new SocketInfo Enhancer
func NewSocketInfoEnhancer(expire, cleanup time.Duration) *SocketInfoEnhancer {
	s := &SocketInfoEnhancer{
		cacheUDP: newSocketCache(expire, cleanup),
		cacheTCP: newSocketCache(expire, cleanup),
	}
	s.scanProc()
	return s
}
