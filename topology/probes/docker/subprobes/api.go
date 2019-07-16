// Copyright (c) 2019 PANTHEON.tech s.r.o.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package subprobes contains API for probes extending topology of docker probe. These probes are handled by docker probe, hence called subprobes.
package subprobes

import (
	"github.com/docker/docker/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
)

// Subprobe is probe attached to docker probe that extends docker graph with additional nodes/edges/metadata.
// The lifecycle of subprobe is handled by docker probe. The docker probe provides to subprobes additional
// information/events so that subprobes can further use them (i.e. to extend docker container graph)
type Subprobe interface {
	probe.Probe

	// RegisterContainer is called by docker probe to notify subprobe about container addition detection (registration)
	RegisterContainer(*ContainerRegistrationData) error
	// UnregisterContainer is called by docker probe to notify subprobe about container removal detection (unregistration)
	UnregisterContainer(*ContainerUnregistrationData) error
}

// ContainerRegistrationData is data holder for passing information about container addition detection from docker probe to its subprobes
type ContainerRegistrationData struct {
	// Info is container information retrieved by container inspect
	Info types.ContainerJSON
	// Node is graph node of container
	Node *graph.Node
	// NSRootID is graph ID of container namespace root in graph
	NSRootID graph.Identifier
}

// ContainerUnregistrationData is data holder for passing information about container removal detection from docker probe to its subprobes
type ContainerUnregistrationData struct {
	// Node is graph node of container
	Node *graph.Node
}
