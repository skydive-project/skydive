// +build linux

/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"runtime"

	"github.com/vishvananda/netns"
)

// NetNSContext describes a NameSpace Context switch API
type NetNSContext struct {
	origns netns.NsHandle
	newns  netns.NsHandle
}

// Quit the NameSpace and go back to the original one
func (n *NetNSContext) Quit() error {
	if n != nil {
		if err := netns.Set(n.origns); err != nil {
			return err
		}
		n.newns.Close()
		n.origns.Close()
	}
	return nil
}

// Close the NameSpace
func (n *NetNSContext) Close() {
	if n != nil && n.origns.IsOpen() {
		n.Quit()
	}

	runtime.UnlockOSThread()
}

// NewNetNsContext creates a new NameSpace context base on path
func NewNetNsContext(path string) (*NetNSContext, error) {
	runtime.LockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("Error while getting current ns: %s", err.Error())
	}

	newns, err := netns.GetFromPath(path)
	if err != nil {
		origns.Close()
		return nil, fmt.Errorf("Error while opening %s: %s", path, err.Error())
	}

	if err = netns.Set(newns); err != nil {
		newns.Close()
		origns.Close()
		return nil, fmt.Errorf("Error while switching from root ns to %s: %s", path, err.Error())
	}

	return &NetNSContext{
		origns: origns,
		newns:  newns,
	}, nil
}
