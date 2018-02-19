// +build !linux

/*
 * Copyright (C) 2018 Iain Grant
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

package lxd

import (
	"github.com/skydive-project/skydive/common"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
)

// LxdProbe describes a Lxd topology graph that enhance the graph
type LxdProbe struct{}

// Start the probe
func (u *LxdProbe) Start() {}

// Stop the probe
func (u *LxdProbe) Stop() {}

// NewLxdProbe creates a new topology Lxd probe
func NewLxdProbe(nsProbe *ns.NetNSProbe, lxdURL string) (*LxdProbe, error) {
	return nil, common.ErrNotImplemented
}
