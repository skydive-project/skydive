// +build !linux

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

package socketinfo

import (
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/probe"
	tp "github.com/skydive-project/skydive/topology/probes"
)

// ProbeHandler describes a probe that collects active connections
type ProbeHandler struct {
}

// Start the socket info probe
func (s *ProbeHandler) Start() {
}

// Stop the socket info probe
func (s *ProbeHandler) Stop() {
}

// Init initializes a new SocketInfo Probe
func (s *ProbeHandler) Init(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	return nil, common.ErrNotImplemented
}
