/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package graph

import (
	"reflect"

	"github.com/skydive-project/skydive/common"
)

// MetadataTransaction describes a metadata transaction in the graph
type MetadataTransaction struct {
	graph        *Graph
	graphElement interface{}
	adds         map[string]interface{}
	removes      []string
}

// AddMetadata in the current transaction
func (t *MetadataTransaction) AddMetadata(k string, v interface{}) {
	t.adds[k] = v
}

// DelMetadata in the current transaction
func (t *MetadataTransaction) DelMetadata(k string) {
	t.removes = append(t.removes, k)
}

// Commit the current transaction to the graph
func (t *MetadataTransaction) Commit() error {
	var e *graphElement
	var kind graphEventType

	switch t.graphElement.(type) {
	case *Node:
		e = &t.graphElement.(*Node).graphElement
		kind = NodeUpdated
	case *Edge:
		e = &t.graphElement.(*Edge).graphElement
		kind = EdgeUpdated
	}

	var updated bool
	for k, v := range t.adds {
		if o, ok := e.Metadata[k]; ok && reflect.DeepEqual(o, v) {
			continue
		}

		if e.Metadata.SetField(k, v) {
			updated = true
		}
	}

	for _, k := range t.removes {
		updated = common.DelField(e.Metadata, k) || updated
	}
	if !updated {
		return nil
	}

	e.UpdatedAt = TimeUTC()
	e.Revision++

	if err := t.graph.backend.MetadataUpdated(t.graphElement); err != nil {
		return err
	}

	t.graph.eventHandler.NotifyEvent(kind, t.graphElement)

	return nil
}
