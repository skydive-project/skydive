// +build linux,!ebpf

/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/graffiti/graph"
)

// NewSocketInfoProbe create a new socket info probe
func NewSocketInfoProbe(g *graph.Graph, host *graph.Node) *ProcSocketInfoProbe {
	return NewProcSocketInfoProbe(g, host)
}
