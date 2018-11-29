/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package common

import (
	"fmt"
	"io/ioutil"
	"os"
)

// ProcessState describes the state of a process
type ProcessState rune

const (
	// Running state
	Running ProcessState = 'R'
	// Sleeping state
	Sleeping ProcessState = 'S'
	// Waiting state
	Waiting ProcessState = 'D'
	// Zombie state
	Zombie ProcessState = 'Z'
	// Stopped state
	Stopped ProcessState = 'T'
	// TracingStop state
	TracingStop ProcessState = 't'
	// Dead state
	Dead ProcessState = 'X'
)

// ProcessInfo describes the information of a running process
type ProcessInfo struct {
	Process  string
	Pid      int64
	Name     string
	PPid     int64
	PGrp     int64
	Session  int64
	Utime    int64
	Stime    int64
	CUtime   int64
	CStime   int64
	Priority int64
	Nice     int64
	Threads  int64
	Start    int64
	Vsize    int64
	RSS      int64
	State    ProcessState
}

// GetProcessInfo retrieve process info from /proc
func GetProcessInfo(pid int) (*ProcessInfo, error) {
	processPath := fmt.Sprintf("/proc/%d", pid)
	exe, err := os.Readlink(processPath + "/exe")
	if err != nil {
		return nil, err
	}

	stat, err := ioutil.ReadFile(processPath + "/stat")
	if err != nil {
		return nil, err
	}

	pi := &ProcessInfo{
		Process: exe,
	}

	var name string
	var state rune
	var null int64

	fmt.Sscanf(string(stat), "%d %s %c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&pi.Pid, &name, &state, &pi.PPid, &pi.PGrp, &pi.Session,
		&null, &null, &null, &null, &null, &null, &null,
		&pi.Utime, &pi.Stime, &pi.CUtime, &pi.CStime, &pi.Priority, &pi.Nice, &pi.Threads,
		&null,
		&pi.Start, &pi.Vsize, &pi.RSS)

	pi.Name = name[1 : len(name)-1]
	pi.State = ProcessState(state)

	return pi, nil
}
