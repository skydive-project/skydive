// +build !packetinject

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package packetinjector

import (
	"fmt"

	"github.com/skydive-project/skydive/ondemand"

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/graph"
)

func (o *onDemandPacketInjectServer) CreateTask(srcNode *graph.Node, resource rest.Resource) (ondemand.Task, error) {
	return nil, fmt.Errorf("packet injection %s ignored on %s because skydive was not built with \"packetinject\" build tag", resource.ID(), srcNode.ID)
}

func (o *onDemandPacketInjectServer) RemoveTask(n *graph.Node, resource rest.Resource, task ondemand.Task) error {
	return nil
}
