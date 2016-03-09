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

package graph

const index = `
mgmt = graph.openManagement()
idKey = mgmt.getPropertyKey("_ID")
if (!idKey) {
  idKey = mgmt.makePropertyKey("_ID").dataType(String.class).make()
}
if (!mgmt.getGraphIndex("byNodeID")) {
  mgmt.buildIndex('byNodeID',Vertex.class).addKey(idKey).unique().buildCompositeIndex()
}
if (!mgmt.getGraphIndex("byEdgeID")) {
  mgmt.buildIndex('byEdgeID',Edge.class).addKey(idKey).buildCompositeIndex()
}
mgmt.commit()
`

type TitangraphBackend struct {
	GremlinBackend
}

func (t *TitangraphBackend) initialize() error {
	_, err := t.client.Query(index)
	if err != nil {
		return err
	}
	return nil
}

func NewTitangraphBackend(addr string, port int) (*TitangraphBackend, error) {
	g, err := NewGremlinBackend(addr, port)
	if err != nil {
		return nil, err
	}

	t := &TitangraphBackend{*g}

	err = t.initialize()
	if err != nil {
		return nil, err
	}

	return t, nil
}
