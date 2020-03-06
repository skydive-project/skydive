/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package process

import (
	"fmt"
	"io/ioutil"
	"os"
)

// State describes the state of a process
type State rune

const (
	// Running state
	Running State = 'R'
	// Sleeping state
	Sleeping State = 'S'
	// Waiting state
	Waiting State = 'D'
	// Zombie state
	Zombie State = 'Z'
	// Stopped state
	Stopped State = 'T'
	// TracingStop state
	TracingStop State = 't'
	// Dead state
	Dead State = 'X'
)

// Info describes the information of a running process
type Info struct {
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
	State    State
}

// GetInfo retrieve process info from /proc
func GetInfo(pid int) (*Info, error) {
	processPath := fmt.Sprintf("/proc/%d", pid)
	exe, err := os.Readlink(processPath + "/exe")
	if err != nil {
		return nil, err
	}

	stat, err := ioutil.ReadFile(processPath + "/stat")
	if err != nil {
		return nil, err
	}

	pi := &Info{
		Process: exe,
	}

	var name string
	var state rune
	var null int64

	_, err = fmt.Sscanf(string(stat), "%d %s %c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&pi.Pid, &name, &state, &pi.PPid, &pi.PGrp, &pi.Session,
		&null, &null, &null, &null, &null, &null, &null,
		&pi.Utime, &pi.Stime, &pi.CUtime, &pi.CStime, &pi.Priority, &pi.Nice, &pi.Threads,
		&null,
		&pi.Start, &pi.Vsize, &pi.RSS)
	if err != nil {
		return nil, err
	}
	pi.Name = name[1 : len(name)-1]
	pi.State = State(state)

	return pi, nil
}
