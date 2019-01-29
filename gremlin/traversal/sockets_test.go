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

package traversal

import (
	"testing"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
)

func TestSocketsIndexer(t *testing.T) {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	g := graph.NewGraph("testhost", b, common.UnknownService)
	indexer := NewSocketIndexer(g)
	indexer.Start()

	m := graph.Metadata{
		"Type": "host",
		"Sockets": []interface{}{
			map[string]interface{}{
				"LocalAddress":  "127.0.0.1",
				"LocalPort":     1234,
				"RemoteAddress": "127.0.0.1",
				"RemotePort":    5678,
			},
		},
	}

	n1, _ := g.NewNode(graph.GenID(), m, "host")

	conn := &socketinfo.ConnectionInfo{
		LocalAddress:  "127.0.0.1",
		LocalPort:     1234,
		RemoteAddress: "127.0.0.1",
		RemotePort:    5678,
	}

	nodes, _ := indexer.FromHash(conn.Hash())
	if len(nodes) != 1 || nodes[0].ID != n1.ID {
		t.Errorf("Must return one node, got %+v", nodes)
	}

	g.NewNode(graph.GenID(), m, "host")
	nodes, _ = indexer.FromHash(conn.Hash())
	if len(nodes) != 2 {
		t.Errorf("Must return two nodes")
	}
}
